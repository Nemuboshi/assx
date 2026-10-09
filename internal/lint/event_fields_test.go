package lint

import (
	"strings"
	"testing"

	"assx/internal/ass"
)

// eventDocument builds a minimal file whose [Events] section uses the given
// Format line (omit it with an empty format) and Dialogue bodies.
func eventDocument(format string, dialogues ...string) ass.Document {
	var builder strings.Builder
	builder.WriteString("[Script Info]\nPlayResX: 640\nPlayResY: 480\n")
	builder.WriteString("[Events]\n")
	if format != "" {
		builder.WriteString("Format: " + format + "\n")
	}
	for _, dialogue := range dialogues {
		builder.WriteString("Dialogue: " + dialogue + "\n")
	}
	return ass.Parse(builder.String())
}

func standardEventDialogue(start, end, text string) string {
	return "0," + start + "," + end + ",Default,,0,0,0,," + text
}

func TestCustomEventFormatIsFlagged(t *testing.T) {
	cases := []struct {
		name   string
		format string
		detail string
	}{
		{name: "standard order is clean", format: "Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text"},
		{name: "ssa order is clean", format: "Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text"},
		{name: "actor alias is clean", format: "Layer, Start, End, Style, Actor, MarginL, MarginR, MarginV, Effect, Text"},
		{name: "space and case variance is clean", format: "layer,  START,End, style,Name,MARGINL ,MarginR, MarginV ,Effect,Text "},
		{name: "reordered fields diverge", format: "Layer, Start, End, Style, Name, Effect, MarginL, MarginR, MarginV, Text", detail: "mis-assigns fields"},
		{name: "short format diverges", format: "Layer, Start, End, Style, Text", detail: "mis-assigns fields"},
		{name: "format without text drops events", format: "Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect", detail: "discards"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := eventDocument(test.format, standardEventDialogue("0:00:00.00", "0:00:02.00", "text"))
			findings := diagnosticsForIDs(AnalyzeEventFields(doc), IssueEventFormat)
			if len(findings) == 0 {
				if test.detail != "" {
					t.Fatalf("custom format produced no ASS024 finding")
				}
				return
			}
			if test.detail == "" {
				t.Fatalf("clean format produced findings: %#v", findings)
			}
			finding := findings[0]
			if !strings.Contains(finding.Detail, test.detail) {
				t.Fatalf("detail = %q, want to mention %q", finding.Detail, test.detail)
			}
			if finding.Severity != Warning || finding.Field != "Format" || finding.FixSafety != "" || len(finding.Edits) != 0 {
				t.Fatalf("finding = %#v", finding)
			}
			// libass drops events when no Format name maps to Text;
			// ass.Parse mirrors that here.
			dropped := test.detail == "discards"
			if parsed := len(doc.Dialogues) > 0; parsed == dropped {
				t.Fatalf("dialogues parsed = %t, want parsed = %t", parsed, !dropped)
			}
			lines := strings.Split(doc.Text, "\n")
			if finding.Line < 1 || finding.Line > len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[finding.Line-1]), "Format:") {
				t.Fatalf("finding anchored at line %d, want the Format line", finding.Line)
			}
		})
	}
}

func TestCustomEventFormatWithoutTextStillParsesNothingElse(t *testing.T) {
	doc := eventDocument("Layer, Start, End, Style", "0,0:00:00.00,0:00:02.00,Default")
	if len(doc.Dialogues) != 0 {
		t.Fatalf("dialogues = %#v, want libass-style drop", doc.Dialogues)
	}
	if findings := diagnosticsForIDs(AnalyzeEventFields(doc), IssueShortEvent, IssueTimecode); len(findings) != 0 {
		t.Fatalf("dropped events produced field findings: %#v", findings)
	}
}

func TestShortEventLineIsFlagged(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		count int
	}{
		{name: "ends before text", body: "0,0:00:00.00,0:00:02.00,Default,,0,0,0,", count: 1},
		{name: "ends after trailing comma", body: "0,0:00:00.00,0:00:02.00,Default,,0,0,0,,", count: 0},
		{name: "empty text cell is complete", body: "0,0:00:00.00,0:00:02.00,Default,,0,0,0,,x", count: 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			line := "Dialogue: " + test.body
			doc := eventDocument("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text", test.body)
			findings := diagnosticsForIDs(AnalyzeEventFields(doc), IssueShortEvent)
			if len(findings) != test.count {
				t.Fatalf("line %q findings = %#v, want %d", line, findings, test.count)
			}
			if test.count == 0 {
				return
			}
			finding := findings[0]
			if finding.Severity != Error || finding.FixSafety != "" || len(finding.Edits) != 0 {
				t.Fatalf("short event finding = %#v", finding)
			}
			if finding.Line != 6 || finding.Column != strings.Index(line, "0,")+1 {
				t.Fatalf("location = %d:%d, want 6:%d", finding.Line, finding.Column, strings.Index(line, "0,")+1)
			}
			if other := diagnosticsForIDs(AnalyzeEventFields(doc), IssueTimecode, IssueEventLayer, IssueKaraoke); len(other) != 0 {
				t.Fatalf("truncated event also drew field findings: %#v", other)
			}
		})
	}
}

func TestTimecodeShape(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{name: "standard shape", body: standardEventDialogue("0:00:01.50", "0:00:02.75", "text"), want: 0},
		{name: "shorthand minutes", body: "0,0,1,Default,,0,0,0,,text", want: 2},
		{name: "missing dot", body: standardEventDialogue("0:00:01", "0:00:02", "text"), want: 2},
		{name: "hours no leading zero", body: standardEventDialogue("1:2:3.4", "0:00:02.00", "text"), want: 0},
		{name: "dash separator", body: standardEventDialogue("0-00-00.00", "0:00:02.00", "text"), want: 1},
		{name: "empty cells skip duration rule", body: standardEventDialogue("", "0:00:02.00", "text"), want: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := eventDocument("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text", test.body)
			findings := diagnosticsForIDs(AnalyzeEventFields(doc), IssueTimecode)
			if len(findings) != test.want {
				t.Fatalf("findings = %#v, want %d", findings, test.want)
			}
			for _, finding := range findings {
				if finding.Severity != Error || finding.FixSafety != "" || len(finding.Edits) != 0 {
					t.Fatalf("timecode finding = %#v", finding)
				}
			}
		})
	}
}

func TestTimecodeColumnsPointAtTheirCells(t *testing.T) {
	body := standardEventDialogue("0:00:1", "bad", "text")
	doc := eventDocument("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text", body)
	findings := diagnosticsForIDs(AnalyzeEventFields(doc), IssueTimecode)
	if len(findings) != 2 {
		t.Fatalf("findings = %#v", findings)
	}
	columns := map[string]int{}
	for _, finding := range findings {
		columns[finding.Field] = finding.Column
	}
	lines := strings.Split(doc.Text, "\n")
	text := lines[len(lines)-2]
	if columns["start"] != strings.Index(text, "0:00:1")+1 || columns["end"] != strings.Index(text, "bad")+1 {
		t.Fatalf("columns = %v in line %q", columns, text)
	}
}

func TestEventDurationFindings(t *testing.T) {
	cases := []struct {
		name       string
		start, end string
		want       int
		detail     string
	}{
		{name: "normal", start: "0:00:00.00", end: "0:00:02.00"},
		{name: "zero duration", start: "0:00:01.00", end: "0:00:01.00", want: 1, detail: "never renders"},
		{name: "reversed", start: "0:00:02.00", end: "0:00:01.00", want: 1, detail: "drops the entry"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			doc := eventDocument("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text", standardEventDialogue(test.start, test.end, "text"))
			findings := diagnosticsForIDs(AnalyzeEventFields(doc), IssueEventDuration)
			if len(findings) != test.want {
				t.Fatalf("findings = %#v, want %d", findings, test.want)
			}
			if test.want == 0 {
				return
			}
			finding := findings[0]
			if finding.Severity != Warning || !strings.Contains(finding.Detail, test.detail) || finding.FixSafety != "" || len(finding.Edits) != 0 {
				t.Fatalf("duration finding = %#v", finding)
			}
		})
	}
}

func TestLayerFieldValues(t *testing.T) {
	cases := []struct {
		layer string
		want  int
	}{
		{layer: "0"}, {layer: "-1"}, {layer: "4294967295"}, {layer: "&h10"}, {layer: "0x2"}, {layer: "&HFF"},
		{layer: "", want: 1},
		{layer: " 3 "},
		{layer: "2.5", want: 1},
		{layer: "3x", want: 1},
		{layer: "two", want: 1},
	}
	for _, test := range cases {
		t.Run("layer/"+test.layer, func(t *testing.T) {
			body := test.layer + ",0:00:00.00,0:00:02.00,Default,,0,0,0,,text"
			doc := eventDocument("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text", body)
			findings := diagnosticsForIDs(AnalyzeEventFields(doc), IssueEventLayer)
			if len(findings) != test.want {
				t.Fatalf("findings = %#v, want %d", findings, test.want)
			}
			for _, finding := range findings {
				if finding.Severity != Error || finding.FixSafety != "" || len(finding.Edits) != 0 {
					t.Fatalf("layer finding = %#v", finding)
				}
				if test.layer == "" && !strings.Contains(finding.Detail, "empty") {
					t.Fatalf("empty layer detail = %q", finding.Detail)
				}
			}
		})
	}
}

func TestEffectFieldFindings(t *testing.T) {
	cases := []struct {
		name    string
		effect  string
		want    int
		detail  string
		quietOK bool
	}{
		{name: "empty", effect: ""},
		{name: "banner with params", effect: "Banner;50;200"},
		{name: "scroll down with params", effect: "Scroll down;50;100;500"},
		{name: "scroll up with params", effect: "Scroll up;50;100;500"},
		{name: "unknown effect", effect: "Fade;", want: 1, detail: "no-op"},
		{name: "banner without semicolon", effect: "Banner", want: 1, detail: "no-op"},
		{name: "case divergence", effect: "banner;50", want: 1, detail: "VSFilter"},
		{name: "case divergence scroll", effect: "SCROLL UP;1;2;3", want: 1, detail: "VSFilter"},
		{name: "banner without params", effect: "Banner;", want: 1},
		{name: "scroll missing params", effect: "Scroll up;50;100", want: 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			body := "0,0:00:00.00,0:00:02.00,Default,,0,0,0," + test.effect + ",text"
			doc := eventDocument("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text", body)
			findings := diagnosticsForIDs(AnalyzeEventFields(doc), IssueEffectField)
			if len(findings) != test.want {
				t.Fatalf("findings = %#v, want %d", findings, test.want)
			}
			for _, finding := range findings {
				if finding.FixSafety != "" || len(finding.Edits) != 0 {
					t.Fatalf("effect finding has automatic fix: %#v", finding)
				}
				if test.detail != "" && !strings.Contains(finding.Detail, test.detail) {
					t.Fatalf("detail = %q, want to mention %q", finding.Detail, test.detail)
				}
			}
		})
	}
}

func TestKaraokeCursorFindings(t *testing.T) {
	cases := []struct {
		name string
		text string
		want int
	}{
		{name: "fits exactly", text: `{\k100}A`, want: 0},
		{name: "one over", text: `{\k100\k100\k10}A`, want: 1},
		{name: "uppercase K counted", text: `{\K300}A`, want: 1},
		{name: "kf and ko counted", text: `{\kf100\ko100\k10}A`, want: 1},
		{name: "default durations", text: `{\k}{\k}{\k}A`, want: 1},
		{name: "zero-width syllable adds nothing", text: `{\k100\k100\k0}A`, want: 0},
		{name: "kt bail", text: `{\kt100}{\k1000}A`, want: 0},
		{name: "transition bail", text: `{\t(0,500,\k100)}{\k100\k100}A`, want: 0},
		{name: "negative bail", text: `{\k-10}{\k100\k100}A`, want: 0},
		{name: "unparsable argument bail", text: `{\k99999999999999999999}{\k100\k100\k100}A`, want: 0},
		{name: "kabc is not karaoke", text: `{\kabc}A`, want: 0},
		{name: "no karaoke", text: `{\i1}A`, want: 0},
		{name: "zero duration event stays quiet", text: `{\k1000}A`, want: 0},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			start, end := "0:00:00.00", "0:00:02.00"
			if test.name == "zero duration event stays quiet" {
				start, end = "0:00:01.00", "0:00:01.00"
			}
			body := standardEventDialogue(start, end, test.text)
			doc := eventDocument("Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text", body)
			findings := diagnosticsForIDs(AnalyzeEventFields(doc), IssueKaraoke)
			if len(findings) != test.want {
				t.Fatalf("findings = %#v, want %d", findings, test.want)
			}
			for _, finding := range findings {
				if finding.Severity != Warning || finding.FixSafety != "" || len(finding.Edits) != 0 {
					t.Fatalf("karaoke finding = %#v", finding)
				}
				if !strings.Contains(finding.Detail, "ms") {
					t.Fatalf("karaoke detail lacks timing: %q", finding.Detail)
				}
			}
		})
	}
}

func TestEventRulesRunThroughAnalyzeDocument(t *testing.T) {
	doc := eventDocument("Layer, Start, End, Style, Text", "0,0,1,Default,reversed")
	findings := AnalyzeDocument(doc)
	ids := map[string]int{}
	for _, finding := range findings {
		ids[finding.ID]++
	}
	if ids[IssueEventFormat] != 1 || ids[IssueTimecode] != 2 || ids[IssueShortEvent] != 0 {
		t.Fatalf("wired diagnostics = %#v", findings)
	}
	// Reversed event without Text field name: libass drops events, so ASS027
	// never sees it; here Text exists but Start>End triggers duration only
	// when both cells parse, and the shorthand cells above do not.
	if ids[IssueEventDuration] != 0 {
		t.Fatalf("unparsable timecodes leaked a duration finding: %#v", findings)
	}
}

func TestNewEventRulesAreReportedWithoutFixes(t *testing.T) {
	for _, id := range []string{IssueEventFormat, IssueShortEvent, IssueTimecode, IssueEventDuration, IssueEventLayer, IssueEffectField, IssueKaraoke} {
		rule, ok := Rules[id]
		if !ok {
			t.Fatalf("rule %s is not registered", id)
		}
		if rule.FixSafety != "" {
			t.Fatalf("rule %s advertises a fix safety: %q", id, rule.FixSafety)
		}
		if len(rule.Sources) == 0 {
			t.Fatalf("rule %s has no pinned renderer sources", id)
		}
	}
}
