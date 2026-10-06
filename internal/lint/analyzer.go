package lint

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"assx/internal/ass"
)

type TextEdit struct {
	Start       int    `json:"start"`
	End         int    `json:"end"`
	Replacement string `json:"replacement"`
}

type Diagnostic struct {
	File        string     `json:"file"`
	ID          string     `json:"id"`
	Severity    Severity   `json:"severity"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Fix         string     `json:"suggested_fix"`
	FixSafety   FixSafety  `json:"fix_safety,omitempty"`
	Edits       []TextEdit `json:"edits,omitempty"`
	Line        int        `json:"line"`
	Column      int        `json:"column"`
	Tag         string     `json:"tag,omitempty"`
	Field       string     `json:"field,omitempty"`
	Detail      string     `json:"detail,omitempty"`
	Sources     []string   `json:"sources"`
}

type owner struct {
	index int
	start int
}

type machine struct {
	line              int
	textStart         int
	position          int
	drawing           bool
	tagIndex          int
	active            map[string]owner
	latched           map[string]int
	allTags           []Tag
	live              map[int]bool
	candidates        map[int]pendingLint
	diagnostics       []Diagnostic
	hasVSFilterModTag bool
}

type pendingLint struct {
	tag    Tag
	detail string
}

var (
	integerPrefix = regexp.MustCompile(`^\s*([+-]?\d+)`)
	numberPrefix  = regexp.MustCompile(`^\s*([+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?)`)
	hexPrefix     = regexp.MustCompile(`^\s*([+-]?[0-9a-fA-F]+)`)
)

func Analyze(dialogue ass.Dialogue) []Diagnostic {
	m := machine{
		line: dialogue.Line, textStart: dialogue.TextStart, active: map[string]owner{}, latched: map[string]int{},
		live: map[int]bool{}, candidates: map[int]pendingLint{},
	}
	for _, token := range Lex(dialogue.Text) {
		if token.Tag == nil {
			m.consumeText(token.Text)
		} else {
			m.consumeTag(*token.Tag)
		}
	}
	for index, pending := range m.candidates {
		if !m.live[index] {
			m.add(IssueNoEffect, pending.tag, pending.detail)
			m.diagnostics[len(m.diagnostics)-1].Edits = []TextEdit{{
				Start: m.textStart + pending.tag.Start, End: m.textStart + pending.tag.End,
			}}
		}
	}
	m.diagnostics = append(m.diagnostics, analyzeRepeatedOpenBraces(dialogue)...)
	if m.hasVSFilterModTag {
		for i := range m.diagnostics {
			m.diagnostics[i].FixSafety = ""
			m.diagnostics[i].Edits = nil
		}
	}
	return m.diagnostics
}

func analyzeRepeatedOpenBraces(dialogue ass.Dialogue) []Diagnostic {
	text := dialogue.Text
	var diagnostics []Diagnostic
	for offset := 0; offset < len(text); {
		open := nextUnescapedOpenBrace(text, offset)
		if open < 0 {
			break
		}
		close := strings.IndexByte(text[open+1:], '}')
		if close < 0 {
			break
		}
		close += open + 1
		runEnd := open + 1
		for runEnd < len(text) && text[runEnd] == '{' {
			runEnd++
		}
		if runEnd > open+1 {
			rule := Rules[IssueRepeatedOpenBrace]
			diagnostics = append(diagnostics, Diagnostic{
				ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
				Fix: rule.Fix, FixSafety: rule.FixSafety, Line: dialogue.Line, Column: open + 2,
				Edits:   []TextEdit{{Start: dialogue.TextStart + open + 1, End: dialogue.TextStart + runEnd}},
				Sources: rule.Sources,
			})
		}
		offset = close + 1
	}
	return diagnostics
}

func nextUnescapedOpenBrace(text string, start int) int {
	for i := start; i < len(text); i++ {
		if text[i] == '\\' && i+1 < len(text) && text[i+1] == '{' {
			i++
			continue
		}
		if text[i] == '{' {
			return i
		}
	}
	return -1
}

func (m *machine) add(id string, tag Tag, detail string) {
	rule := Rules[id]
	m.diagnostics = append(m.diagnostics, Diagnostic{
		ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
		Fix: rule.Fix, FixSafety: rule.FixSafety, Line: m.line, Column: tag.Column, Tag: tag.Name,
		Detail: detail, Sources: rule.Sources,
	})
}

func (m *machine) consumeText(text string) {
	if text == "" {
		return
	}
	if m.drawing {
		m.markActiveLive()
		m.position++
		return
	}
	for i := 0; i < len(text); {
		if text[i] == '\\' && i+1 < len(text) && strings.ContainsRune("Nnh{}", rune(text[i+1])) {
			m.markActiveLive()
			m.position++
			i += 2
			continue
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		m.markActiveLive()
		m.position++
		i += size
	}
}

func (m *machine) markActiveLive() {
	for _, active := range m.active {
		if active.index >= 0 {
			m.live[active.index] = true
		}
	}
}

func (m *machine) consumeTag(tag Tag) {
	index := m.tagIndex
	m.tagIndex++
	m.allTags = append(m.allTags, tag)
	if tag.RepeatedSlashes > 0 {
		m.add(IssueRepeatedSlash, tag, fmt.Sprintf("Found %d extra backslash(es) before the tag.", tag.RepeatedSlashes))
		m.diagnostics[len(m.diagnostics)-1].Edits = []TextEdit{{
			Start: m.textStart + tag.Start, End: m.textStart + tag.Start + tag.RepeatedSlashes,
		}}
		return
	}
	m.validate(tag)
	spec, known := TagSpecs[tag.Name]
	if !known {
		m.add(IssueUnknownTag, tag, "Unknown override tag.")
		return
	}
	if tag.Name == "N" || tag.Name == "n" || tag.Name == "h" {
		m.add(IssueUnknownTag, tag, "This text escape is only valid in dialogue text, outside an override block.")
		return
	}
	if spec.VSFilterModOnly {
		m.hasVSFilterModTag = true
		m.add(IssueVSFilterModTag, tag, "This tag is specific to VSFilterMod and is not shared by libass and VSFilter. Automatic fixes are disabled for this dialogue.")
		return
	}
	if spec.Counts != nil && !contains(spec.Counts, len(tag.Args)) {
		return
	}
	if spec.Behavior == StyleReset {
		m.resetStyle()
		return
	}
	if spec.Behavior == Transition || spec.Behavior == Accumulate {
		return
	}

	slots, behavior := spec.Slots, spec.Behavior
	if tag.Name == "clip" || tag.Name == "iclip" {
		if len(tag.Args) == 4 {
			slots, behavior = []string{"clip_rect"}, Assign
		} else {
			slots, behavior = []string{"clip_vector"}, FirstWins
		}
	}
	for _, slot := range slots {
		if behavior == FirstWins {
			if first, exists := m.latched[slot]; exists {
				m.candidates[index] = pendingLint{tag: tag, detail: fmt.Sprintf("Ignored because an earlier tag with index %d owns this first-wins slot.", first)}
				continue
			}
			m.latched[slot] = index
		}
		if previous, exists := m.active[slot]; exists && previous.index >= 0 && previous.start == m.position && !tag.InTransition {
			m.candidates[previous.index] = pendingLint{tag: m.tagAt(previous.index), detail: "Overwritten before any dialogue text used it."}
		}
		m.active[slot] = owner{index: index, start: m.position}
		if slot == "drawing_scale" {
			value := 0
			if len(tag.Args) > 0 {
				value, _ = parseInteger(tag.Args[0])
			}
			m.drawing = value > 0
		}
	}
}

func (m *machine) tagAt(index int) Tag {
	if index >= 0 && index < len(m.allTags) {
		return m.allTags[index]
	}
	return Tag{}
}

func (m *machine) resetStyle() {
	for slot, previous := range m.active {
		if KeepOnStyleReset[slot] {
			continue
		}
		if _, firstWins := m.latched[slot]; firstWins {
			continue
		}
		if previous.index >= 0 && previous.start == m.position {
			m.candidates[previous.index] = pendingLint{tag: m.tagAt(previous.index), detail: "Reset before any dialogue text used it."}
		}
		m.active[slot] = owner{index: -1, start: m.position}
	}
}

func (m *machine) validate(tag Tag) {
	spec, known := TagSpecs[tag.Name]
	if !known {
		return
	}
	if spec.Counts != nil && !contains(spec.Counts, len(tag.Args)) {
		m.add(IssueArgumentCount, tag, fmt.Sprintf("Found %d arguments; expected %s.", len(tag.Args), countsText(spec.Counts)))
		return
	}
	if len(tag.Args) == 0 {
		return
	}
	arg := tag.Args[0]
	invalid := func(detail string) { m.add(IssueInvalidValue, tag, detail) }
	switch spec.Value {
	case IntegerValue:
		value, ok := parseInteger(arg)
		if !ok {
			invalid(fmt.Sprintf("Expected an integer, found %q.", arg))
		} else if (spec.Min != 0 || spec.Max != 0) && (value < spec.Min || value > spec.Max) {
			invalid(fmt.Sprintf("Value %d is outside the accepted range %d..%d.", value, spec.Min, spec.Max))
		}
	case NumberValue:
		if !numberPrefix.MatchString(arg) {
			invalid(fmt.Sprintf("Expected a number, found %q.", arg))
		}
		if tag.Name == "blur" {
			if value, ok := parseNumber(arg); ok && value > 100 {
				m.add(IssueRendererDiff, tag, "Values above 100 are clamped by libass but have no matching upper clamp in VSFilter.")
			}
		}
	case BoldValue:
		value, ok := parseInteger(arg)
		if !ok || (value != 0 && value != 1 && value < 100) {
			invalid(fmt.Sprintf("Bold value %q must be 0, 1, or at least 100.", arg))
		}
	case FontNameValue:
		if tag.Paren && len(tag.Args) > 1 {
			m.add(IssueFontComma, tag, "A comma inside parenthesized \\fn syntax separates arguments.")
		}
	case HexValue:
		value := arg
		if !tag.Paren {
			value = strings.Trim(value, "&H")
		} else {
			value = strings.TrimLeft(value, "&H")
		}
		if !hexPrefix.MatchString(value) {
			invalid(fmt.Sprintf("Expected a hexadecimal value, found %q.", arg))
		} else if tag.Paren && strings.HasPrefix(strings.ToUpper(arg), "&H") {
			m.add(IssueRendererDiff, tag, "VSFilter and libass treat an &H prefix in parenthesized color/alpha arguments differently.")
		}
	case NumberListValue:
		for _, value := range tag.Args {
			if !numberPrefix.MatchString(value) {
				invalid(fmt.Sprintf("Expected numeric arguments; found %q.", value))
				break
			}
		}
	case RectValue:
		if len(tag.Args) == 4 {
			for _, value := range tag.Args {
				if !numberPrefix.MatchString(value) {
					invalid(fmt.Sprintf("Expected numeric rectangle arguments; found %q.", value))
					break
				}
			}
		}
	}
}

func contains(values []int, want int) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func countsText(values []int) string {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = strconv.Itoa(value)
	}
	return strings.Join(parts, " or ")
}

func parseInteger(value string) (int, bool) {
	match := integerPrefix.FindStringSubmatch(value)
	if len(match) == 0 {
		return 0, false
	}
	parsed, err := strconv.Atoi(match[1])
	return parsed, err == nil
}

func parseNumber(value string) (float64, bool) {
	match := numberPrefix.FindStringSubmatch(value)
	if len(match) == 0 {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(match[1], 64)
	return parsed, err == nil
}
