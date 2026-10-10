// Package perftest contains deterministic workload helpers used only by tests
// and benchmarks. No production package imports it.
package perftest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Workloads are checked-in, hand-curated ASS scenarios with varied dialogues,
// typography, drawing commands, and transforms.
var Workloads = []string{"dialogue", "typesetting", "drawing", "transforms"}

const LongDialogueCount = 10000

// Load reads fixture bytes before benchmark timers start.
// All consuming benchmark packages live one directory below internal/.
func Load(tb testing.TB, name string) string {
	tb.Helper()
	path := filepath.Join("..", "..", "testdata", "perf", name+".ass")
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("read fixture %q: %v", path, err)
	}
	return string(data)
}

// Long deterministically cycles through all curated dialogue lines with one
// shared header. Building the input is always excluded from measured regions.
func Long(tb testing.TB) string {
	tb.Helper()
	first := Load(tb, Workloads[0])
	start := strings.Index(first, "Dialogue: ")
	if start < 0 {
		tb.Fatal("dialogue fixture has no events")
	}

	lines := make([]string, 0, 224)
	for _, name := range Workloads {
		for _, line := range strings.Split(Load(tb, name), "\n") {
			if strings.HasPrefix(line, "Dialogue: ") {
				lines = append(lines, line+"\n")
			}
		}
	}
	if len(lines) == 0 {
		tb.Fatal("performance fixtures have no dialogue lines")
	}

	var out strings.Builder
	out.Grow(start + LongDialogueCount*120)
	out.WriteString(first[:start])
	for i := 0; i < LongDialogueCount; i++ {
		out.WriteString(lines[i%len(lines)])
	}
	return out.String()
}

func Cases(tb testing.TB) []struct {
	Name string
	Text string
} {
	tb.Helper()
	cases := make([]struct {
		Name string
		Text string
	}, 0, len(Workloads)+1)
	for _, name := range Workloads {
		cases = append(cases, struct {
			Name string
			Text string
		}{name, Load(tb, name)})
	}
	cases = append(cases, struct {
		Name string
		Text string
	}{fmt.Sprintf("long_%d", LongDialogueCount), Long(tb)})
	return cases
}
