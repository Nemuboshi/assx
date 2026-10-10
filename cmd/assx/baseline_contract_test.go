package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"assx/internal/ass"
	"assx/internal/lint"
	"assx/internal/semantic"
	"golang.org/x/image/font/gofont/goregular"
)

// These are observations from main@6ba745336615f102d5de39f76550f0a45da7bbb6.
// Update only under the review procedure in docs/renderer-contracts.md.
// The snapshots are not specifications for actual renderer behavior.
const baselineCommit = "6ba745336615f102d5de39f76550f0a45da7bbb6"

var updateContracts = flag.Bool("update-contracts", false, "regenerate renderer baseline golden files (requires an explicit reviewed correction)")

type baselineTag struct {
	Name       string        `json:"name"`
	Raw        string        `json:"raw"`
	Args       []string      `json:"args,omitempty"`
	Start      int           `json:"start"`
	End        int           `json:"end"`
	Column     int           `json:"column"`
	Paren      bool          `json:"paren,omitempty"`
	Transition bool          `json:"transition,omitempty"`
	Slashes    int           `json:"extra_slashes,omitempty"`
	Children   []baselineTag `json:"children,omitempty"`
}

func snapshotTag(tag ass.Tag) baselineTag {
	out := baselineTag{
		Name: tag.Name, Raw: tag.Raw, Args: tag.Args, Start: tag.Start, End: tag.End,
		Column: tag.Column, Paren: tag.Paren, Transition: tag.InTransition,
		Slashes: tag.RepeatedSlashes,
	}
	for _, child := range tag.Children {
		out.Children = append(out.Children, snapshotTag(child))
	}
	return out
}

type baselineItem struct {
	Kind  ass.BlockItemKind `json:"kind"`
	Raw   string            `json:"raw"`
	Start int               `json:"start"`
	End   int               `json:"end"`
	Tag   *baselineTag      `json:"tag,omitempty"`
}

type baselineNode struct {
	Kind  ass.DialogueNodeKind `json:"kind"`
	Raw   string               `json:"raw"`
	Start int                  `json:"start"`
	End   int                  `json:"end"`
	Items []baselineItem       `json:"items,omitempty"`
}

type baselineSlot struct {
	Name   string              `json:"name"`
	Value  semantic.StateValue `json:"value"`
	Source int                 `json:"source"`
}

var observedSlots = []string{
	"fontname", "fontsize", "position", "origin", "alignment", "frz",
	"clip_rect", "clip_vector", "karaoke_cursor", "drawing_scale",
}

func snapshotSlots(view semantic.StateView) []baselineSlot {
	slots := make([]baselineSlot, 0, len(observedSlots))
	for _, name := range observedSlots {
		slots = append(slots, baselineSlot{Name: name, Value: view.Value(name), Source: view.Source(name)})
	}
	return slots
}

type baselineEvent struct {
	Tag         baselineTag                  `json:"tag"`
	Index       int                          `json:"index"`
	Policy      uint8                        `json:"policy"`
	Slots       []string                     `json:"slots,omitempty"`
	Before      []semantic.StateValue        `json:"before,omitempty"`
	After       []semantic.StateValue        `json:"after,omitempty"`
	Applied     bool                         `json:"applied"`
	Known       bool                         `json:"known"`
	Barrier     bool                         `json:"barrier"`
	Uncertainty semantic.SemanticUncertainty `json:"uncertainty"`
	ActiveStyle string                       `json:"active_style"`
	State       []baselineSlot               `json:"state"`
}

type baselineText struct {
	Text        string         `json:"text"`
	Start       int            `json:"start"`
	ActiveStyle string         `json:"active_style"`
	State       []baselineSlot `json:"state"`
}

type baselineEffect struct {
	Tag     baselineTag `json:"tag"`
	Reason  uint8       `json:"reason"`
	Owner   int         `json:"owner"`
	Revoked bool        `json:"revoked"`
}

type baselineDialogue struct {
	Line          int              `json:"line"`
	Style         string           `json:"style"`
	TextStart     int              `json:"text_start"`
	MissingFields bool             `json:"missing_fields"`
	Fields        []ass.EventField `json:"fields"`
	Nodes         []baselineNode   `json:"nodes"`
	Events        []baselineEvent  `json:"events,omitempty"`
	Text          []baselineText   `json:"text_boundaries,omitempty"`
	NoEffects     []baselineEffect `json:"no_effects,omitempty"`
}

func snapshotDialogue(dialogue ass.Dialogue, styles map[string]semantic.StyleState) baselineDialogue {
	tree := dialogue.ParsedText()
	out := baselineDialogue{
		Line: dialogue.Line, Style: dialogue.Style, TextStart: dialogue.TextStart,
		MissingFields: dialogue.MissingFields, Fields: dialogue.Fields,
	}
	for _, node := range tree.Nodes {
		part := baselineNode{Kind: node.Kind, Raw: tree.Source[node.Start:node.End], Start: node.Start, End: node.End}
		if node.Block != nil {
			for _, item := range node.Block.Items {
				bi := baselineItem{Kind: item.Kind, Raw: item.Raw, Start: item.Start, End: item.End}
				if item.Tag != nil {
					tag := snapshotTag(*item.Tag)
					bi.Tag = &tag
				}
				part.Items = append(part.Items, bi)
			}
		}
		out.Nodes = append(out.Nodes, part)
	}
	options := semantic.EvaluationOptions{
		Styles: styles, DialogueStyle: dialogue.Style,
		Observer: semantic.Observer{
			Tag: func(event semantic.TagEvent, view semantic.StateView) {
				e := baselineEvent{
					Tag: snapshotTag(event.Tag), Index: event.Index, Policy: uint8(event.Policy),
					Slots: event.Slots, Applied: event.Applied, Known: event.Known,
					Barrier: event.Barrier, Uncertainty: event.Uncertainty,
					ActiveStyle: view.ActiveStyle(), State: snapshotSlots(view),
				}
				for i := range event.Slots {
					e.Before = append(e.Before, event.Before[i])
					e.After = append(e.After, event.After[i])
				}
				out.Events = append(out.Events, e)
			},
			Text: func(text string, start int, view semantic.StateView) {
				out.Text = append(out.Text, baselineText{
					Text: text, Start: start, ActiveStyle: view.ActiveStyle(),
					State: snapshotSlots(view),
				})
			},
		},
	}
	for _, effect := range semantic.Evaluate(tree, options).NoEffects {
		out.NoEffects = append(out.NoEffects, baselineEffect{
			Tag: snapshotTag(effect.Tag), Reason: uint8(effect.Reason),
			Owner: effect.OwnerIndex, Revoked: effect.ProofRevoked,
		})
	}
	return out
}

type baselineCLI struct {
	ExitCode    int               `json:"exit_code"`
	Diagnostics []lint.Diagnostic `json:"diagnostics"`
	Summary     string            `json:"summary"`
	OutputSHA   string            `json:"output_sha256"`
	Changed     bool              `json:"changed"`
}

func sha(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func snapshotCLI(t *testing.T, filename string, raw []byte, font bool, fixMode string) baselineCLI {
	t.Helper()
	path := filepath.Join(t.TempDir(), filename)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"--format", "json"}
	if font {
		fontDir := filepath.Join(t.TempDir(), "fonts")
		if err := os.MkdirAll(fontDir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(fontDir, "Go-Regular.ttf"), goregular.TTF, 0o600); err != nil {
			t.Fatal(err)
		}
		args = append(args, "--check-fonts", "--font-dir", fontDir)
	}
	if fixMode != "" {
		args = append(args, fixMode)
	}
	args = append(args, path)
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	var diagnostics []lint.Diagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("%s %s: invalid CLI JSON: %v\n%s\n%s", filename, fixMode, err, stdout.String(), stderr.String())
	}
	for i := range diagnostics {
		diagnostics[i].File = filename
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return baselineCLI{
		ExitCode: code, Diagnostics: diagnostics,
		Summary: strings.TrimSpace(stderr.String()), OutputSHA: sha(after),
		Changed: !bytes.Equal(raw, after),
	}
}

type baselineRecord struct {
	SourceCommit string             `json:"source_commit"`
	FixtureSHA   string             `json:"fixture_sha256"`
	RoundTrip    bool               `json:"lossless_round_trip"`
	Newline      string             `json:"newline"`
	Dialogues    []baselineDialogue `json:"dialogues"`
	CLI          baselineCLI        `json:"cli"`
	CLIFonts     baselineCLI        `json:"cli_fonts"`
	CLISafeFix   baselineCLI        `json:"cli_safe_fix"`
	CLIUnsafeFix baselineCLI        `json:"cli_unsafe_fix"`
}

func TestRendererBaselineContracts(t *testing.T) {
	for _, filename := range []string{
		"ordinary.ass", "renderer-collisions.ass", "semantic-barriers.ass",
		"style-fonts.ass", "utf8bom.ass", "utf16le.ass",
	} {
		t.Run(filename, func(t *testing.T) {
			input := filepath.Join("..", "..", "testdata", "contracts", "cases", filename)
			raw, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			source, err := ass.DecodeSource(raw)
			if err != nil {
				t.Fatal(err)
			}
			doc := ass.Parse(source.Text)
			record := baselineRecord{
				SourceCommit: baselineCommit, FixtureSHA: sha(raw),
				RoundTrip: bytes.Equal(source.Encode(doc.Text), raw),
				Newline:   doc.Newline,
			}
			if !record.RoundTrip {
				t.Fatal("fixture is not losslessly round-trippable")
			}
			styles := semantic.StyleStatesByName(doc.StyleFields)
			for _, dialogue := range doc.Dialogues {
				record.Dialogues = append(record.Dialogues, snapshotDialogue(dialogue, styles))
			}
			record.CLI = snapshotCLI(t, filename, raw, false, "")
			record.CLIFonts = snapshotCLI(t, filename, raw, true, "")
			record.CLISafeFix = snapshotCLI(t, filename, raw, false, "--fix")
			record.CLIUnsafeFix = snapshotCLI(t, filename, raw, false, "--unsafe-fix")
			actual, err := json.MarshalIndent(record, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			actual = append(actual, '\n')
			golden := filepath.Join("..", "..", "testdata", "contracts", "golden", filename+".json")
			if *updateContracts {
				if err := os.WriteFile(golden, actual, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden %s: %v (run go test ./cmd/assx -run TestRendererBaselineContracts -update-contracts only when establishing a reviewed baseline)", golden, err)
			}
			if !bytes.Equal(actual, want) {
				t.Errorf("baseline drift in %s: inspect golden diff; do not update without verified evidence and a documented correction (want %s, got %s)", filename, sha(want), sha(actual))
			}
		})
	}
}

func TestBaselinePinnedCommitIsDocumented(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "renderer-contracts.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(doc), baselineCommit) {
		t.Fatalf("baseline source commit %s missing from contract documentation", baselineCommit)
	}
}
