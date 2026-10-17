package analysis

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// Snapshot is one immutable analysis input: an opaque URI or path, its UTF-8
// text, and an optional version. It holds no parsed form and no file handle.
type Snapshot struct {
	uri     string
	text    string
	version *int64
}

// NewSnapshot validates UTF-8 text and a non-empty URI. It copies the
// optional version, so a later change to the caller value cannot alter the
// snapshot.
func NewSnapshot(uri, text string, version *int64) (Snapshot, error) {
	if uri == "" {
		return Snapshot{}, errors.New("analysis snapshot requires a URI or path")
	}
	if !utf8.ValidString(text) {
		return Snapshot{}, fmt.Errorf("analysis snapshot for %s is not valid UTF-8", uri)
	}
	snapshot := Snapshot{uri: uri, text: text}
	if version != nil {
		value := *version
		snapshot.version = &value
	}
	return snapshot, nil
}

// URI returns the opaque input identity.
func (s Snapshot) URI() string { return s.uri }

// Text returns the UTF-8 source.
func (s Snapshot) Text() string { return s.text }

// Version returns a copy of the optional version, or nil.
func (s Snapshot) Version() *int64 {
	if s.version == nil {
		return nil
	}
	value := *s.version
	return &value
}
