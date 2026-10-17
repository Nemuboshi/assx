package perftest

import (
	"fmt"
	"strings"
	"testing"
)

// ScalingCounts are the dialogue counts for source-size scaling workloads.
var ScalingCounts = []int{250, 1000, 4000, 10000}

// StyleEditDialogueCount is the dialogue count in the Style-edit workload.
// Each dialogue carries the established three-tag no-effect block, so one
// aggregate proof covers the Style edit plus every dialogue edit.
const StyleEditDialogueCount = 250

// styleEditHeader uses fixed VSFilter Style slots and a Default Bold of 0
// with Fontsize 20, matching the proof-clean shape of the late-failure
// benchmark document. MarginL carries a fractional integer value, so one
// ASS011 Style edit proves document-wide.
const styleEditHeader = "[Script Info]\nPlayResX: 1920\nPlayResY: 1080\nYCbCr Matrix: None\nLayoutResX: 1920\nLayoutResY: 1080\n" +
	"[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n" +
	"Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,0.0,0,0,1\n" +
	"[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"

// noEffectPrefix carries the established three removable override tags plus
// one animation transform.
const noEffectPrefix = `{` + `\b1\b1\fs20\fs20\t(0,500,\fscx100)}`

// Scaling returns one deterministic document per ScalingCounts entry. Each
// cycles the curated dialogue texts under one shared header, prefixed with
// one no-effect override block. Construction happens before benchmark timing.
func Scaling(tb testing.TB) []struct {
	Name string
	Text string
} {
	tb.Helper()
	header, lines := scalingCommon(tb)
	cases := make([]struct {
		Name string
		Text string
	}, 0, len(ScalingCounts))
	for _, count := range ScalingCounts {
		var out strings.Builder
		out.Grow(len(header) + count*120)
		out.WriteString(header)
		for i := 0; i < count; i++ {
			out.WriteString(scalingDialogueLine(tb, i, lines[i%len(lines)]) + "\n")
		}
		cases = append(cases, struct {
			Name string
			Text string
		}{fmt.Sprintf("scaling_%d", count), out.String()})
	}
	return cases
}

// StyleEdit returns a document whose Default Style carries a fractional
// integer MarginL value (one ASS011 Style edit) and whose dialogues each
// carry one fixed no-effect override block. A Style edit selects every
// dialogue for proof, so this workload measures document-wide SafeFix
// preparation. Construction happens before benchmark timing.
func StyleEdit(tb testing.TB) string {
	tb.Helper()
	var out strings.Builder
	out.Grow(len(styleEditHeader) + StyleEditDialogueCount*120)
	out.WriteString(styleEditHeader)
	const line = "Dialogue: 0,0:00:00.00,0:00:01.00,Default,,0,0,0,,{\\b1\\b1\\fs20\\fs20\\t(0,500,\\fscx100)}Benchmark subtitle"
	for i := 0; i < StyleEditDialogueCount; i++ {
		out.WriteString(line + "\n")
	}
	return out.String()
}

func scalingCommon(tb testing.TB) (string, []string) {
	tb.Helper()
	first := Load(tb, Workloads[0])
	start := strings.Index(first, "Dialogue: ")
	if start < 0 {
		tb.Fatal("dialogue fixture has no events")
	}
	var lines []string
	for _, name := range Workloads {
		for _, line := range strings.Split(Load(tb, name), "\n") {
			if strings.HasPrefix(line, "Dialogue: ") {
				lines = append(lines, line)
			}
		}
	}
	if len(lines) == 0 {
		tb.Fatal("performance fixtures have no dialogue lines")
	}
	return first[:start], lines
}

func scalingDialogueLine(tb testing.TB, index int, line string) string {
	tb.Helper()
	fields := strings.SplitN(line, ",", 10)
	if len(fields) != 10 {
		tb.Fatalf("curated dialogue line is incomplete: %q", line)
	}
	start := index * 2
	fields[1] = formatTimecode(start)
	fields[2] = formatTimecode(start + 1)
	fields[3] = "Default"
	fields[9] = noEffectPrefix + strings.TrimPrefix(fields[9], ",")
	return strings.Join(fields, ",")
}

func formatTimecode(total int) string {
	h, m, s := total/3600, total/60%60, total%60
	return fmt.Sprintf("%d:%02d:%02d.00", h, m, s)
}
