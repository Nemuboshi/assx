package lsp

import (
	"fmt"
	"net/url"
	"unicode/utf16"
	"unicode/utf8"

	"assx/internal/ass"
	"assx/internal/sourcepos"
)

type Change struct {
	Range       *sourcepos.Range `json:"range,omitempty"`
	RangeLength *int             `json:"rangeLength,omitempty"`
	Text        string           `json:"text"`
}

type document struct {
	text    string
	version int
	index   sourcepos.Index
	parsed  ass.Document
}

func newDocument(text string, version int) (document, error) {
	index, err := sourcepos.New(text)
	if err != nil {
		return document{}, err
	}
	return document{text: text, version: version, index: index, parsed: ass.Parse(text)}, nil
}

// Documents keeps current, unsaved text. A session owns its store.
type Documents struct {
	items map[string]document
}

func validURI(uri string) bool {
	parsed, err := url.Parse(uri)
	return err == nil && parsed.Scheme != "" && (parsed.Opaque != "" || parsed.Path != "") && parsed.Fragment == ""
}

func (store *Documents) Open(uri string, version int, text string) error {
	if !validURI(uri) || !utf8.ValidString(text) || len(text) > maxMessageBytes {
		return fmt.Errorf("invalid document URI or text")
	}
	if store.items == nil {
		store.items = make(map[string]document)
	}
	if _, exists := store.items[uri]; exists {
		return fmt.Errorf("document %q is already open", uri)
	}
	current, err := newDocument(text, version)
	if err != nil {
		return err
	}
	store.items[uri] = current
	return nil
}

// Change applies all edits to a copy and commits only a valid, newer version.
func (store *Documents) Change(uri string, version int, changes []Change) error {
	current, exists := store.items[uri]
	if !exists || version <= current.version || len(changes) == 0 {
		return fmt.Errorf("unknown document, stale version, or empty change")
	}
	text := current.text
	index := current.index
	for _, change := range changes {
		if !utf8.ValidString(change.Text) {
			return fmt.Errorf("change text is not valid UTF-8")
		}
		if change.Range == nil {
			text = change.Text
		} else {
			start, err := index.Offset(change.Range.Start)
			if err != nil {
				return err
			}
			end, err := index.Offset(change.Range.End)
			if err != nil || end < start {
				return fmt.Errorf("invalid change range")
			}
			if change.RangeLength != nil && len(utf16.Encode([]rune(text[start:end]))) != *change.RangeLength {
				return fmt.Errorf("change rangeLength does not match the source")
			}
			text = text[:start] + change.Text + text[end:]
		}
		if len(text) > maxMessageBytes {
			return fmt.Errorf("document exceeds the size limit")
		}
		var err error
		index, err = sourcepos.New(text)
		if err != nil {
			return err
		}
	}
	store.items[uri] = document{text: text, version: version, index: index, parsed: ass.Parse(text)}
	return nil
}

func (store *Documents) Snapshot(uri string) (string, int, bool) {
	current, exists := store.items[uri]
	return current.text, current.version, exists
}

func (store *Documents) Close(uri string) {
	delete(store.items, uri)
}
