package lint

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"assx/internal/ass"
	"assx/internal/ass/renderer"
)

// assEventFormat and ssaEventFormat mirror libass ass_event_format and
// ssa_event_format (ass.c:49-58). libass maps event cells by the Format name
// (process_event_tail, ass.c:481-524) while xy-VSFilter and VSFilterMod read
// the same cells at fixed positions and ignore Format entirely
// (STS.cpp:1455-1472), so a Format line that is not one of these two
// canonical orders desynchronizes the engines.
var (
	assEventFormat = []string{"layer", "start", "end", "style", "name", "marginl", "marginr", "marginv", "effect", "text"}
	ssaEventFormat = []string{"marked", "start", "end", "style", "name", "marginl", "marginr", "marginv", "effect", "text"}
)

// eventTimeShape is the four-group timecode both engines honor: H:MM:SS.CS,
// where the fourth segment counts centiseconds (libass string2timecode
// ass.c:249-260 computes ms*10; xy-VSFilter STS.cpp:1457-1462 reads the same
// fixed separators and computes ms_div10*10). A cell that fails the shape
// zeroes the whole timestamp in libass and makes VSFilter throw in NextInt
// and discard the event (STS.cpp:1284-1301).
var eventTimeShape = regexp.MustCompile(`^[+-]?\d+:[+-]?\d+:[+-]?\d+\.[+-]?\d+$`)

// AnalyzeEventFields checks the Events Format line and every parsed event
// field against what libass and VSFilter actually do with it.
func AnalyzeEventFields(doc ass.Document) []Diagnostic {
	return analyzeEventFields(doc, nil, true)
}

// AnalyzeEventFieldsForRenderer keeps the shared document-level checks while
// obtaining karaoke timing from the selected profile, not the legacy tags.
func AnalyzeEventFieldsForRenderer(doc ass.Document, profile renderer.Profile) []Diagnostic {
	return scopeDiagnostics(analyzeEventFields(doc, &profile, true), profile)
}

// includeKaraoke is false only when the document's single shared P05
// evaluation has a karaoke observer attached in analyzeResolvedDialogue.
func analyzeEventFields(doc ass.Document, profile *renderer.Profile, includeKaraoke bool) []Diagnostic {
	diagnostics := eventFormatFindings(doc)
	for _, dialogue := range doc.Dialogues {
		if dialogue.MissingFields {
			rule := Rules[IssueShortEvent]
			column := len("Dialogue:") + 1
			if field, ok := dialogue.Field("layer"); ok {
				column = field.Start - dialogue.LineStart + 1
			} else if field, ok := dialogue.Field("marked"); ok {
				column = field.Start - dialogue.LineStart + 1
			}
			diagnostics = append(diagnostics, Diagnostic{
				ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
				Fix: rule.Fix, Line: dialogue.Line, Column: column, Field: "Dialogue",
				Detail:  "The line ends before all Format fields have a value; libass discards the event and VSFilter stops parsing at the missing cell.",
				Sources: rule.Sources,
			})
			continue
		}
		diagnostics = append(diagnostics, analyzeTimecodes(dialogue)...)
		diagnostics = append(diagnostics, analyzeEventDuration(dialogue)...)
		diagnostics = append(diagnostics, analyzeLayer(dialogue)...)
		diagnostics = append(diagnostics, analyzeEffect(dialogue)...)
		if includeKaraoke {
			if profile == nil {
				diagnostics = append(diagnostics, analyzeKaraoke(dialogue)...)
			} else {
				diagnostics = append(diagnostics, analyzeKaraokeForRenderer(dialogue, *profile)...)
			}
		}
	}
	return diagnostics
}

func eventFormatFindings(doc ass.Document) []Diagnostic {
	var diagnostics []Diagnostic
	rule := Rules[IssueEventFormat]
	for _, raw := range doc.Lines {
		if raw.Section != "events" {
			continue
		}
		trimmed := strings.TrimLeft(raw.Content, " \t")
		if !strings.HasPrefix(strings.ToLower(trimmed), "format:") {
			continue
		}
		format := ass.ParseFormat(trimmed[len("Format:"):])
		if formatMatches(format, assEventFormat) || formatMatches(format, ssaEventFormat) {
			continue
		}
		detail := "libass reads each event field by the Format name while VSFilter reads fixed positions, so this order mis-assigns fields in one of the engines."
		if !ass.HasFormatName(format, "text") {
			detail = "The Format line has no Text field, so libass silently discards every following Dialogue line."
		}
		diagnostics = append(diagnostics, Diagnostic{
			ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
			Fix: rule.Fix, Line: raw.Line, Column: len(raw.Content) - len(trimmed) + 1, Field: "Format",
			Detail: detail, Sources: rule.Sources,
		})
	}
	return diagnostics
}

func formatMatches(format, canonical []string) bool {
	if len(format) != len(canonical) {
		return false
	}
	for i := range format {
		// libass treats the Actor name as an alias for Name
		// (process_event_tail ALIAS(Actor,Name), ass.c:510).
		if format[i] == canonical[i] || (canonical[i] == "name" && format[i] == "actor") {
			continue
		}
		return false
	}
	return true
}

func analyzeTimecodes(dialogue ass.Dialogue) []Diagnostic {
	var diagnostics []Diagnostic
	for _, name := range []string{"start", "end"} {
		field, ok := dialogue.Field(name)
		if !ok {
			continue
		}
		value := strings.TrimSpace(field.Value)
		if eventTimeShape.MatchString(value) {
			continue
		}
		rule := Rules[IssueTimecode]
		diagnostics = append(diagnostics, Diagnostic{
			ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
			Fix: rule.Fix, Line: dialogue.Line, Column: field.Start - dialogue.LineStart + 1, Field: name,
			Detail:  fmt.Sprintf("The %s field %q is not H:MM:SS.CS; libass reads it as 0:00:00.00 and VSFilter discards the event.", name, value),
			Sources: rule.Sources,
		})
	}
	return diagnostics
}

func analyzeEventDuration(dialogue ass.Dialogue) []Diagnostic {
	start, startOK := dialogueTime(dialogue, "start")
	end, endOK := dialogueTime(dialogue, "end")
	if !startOK || !endOK || start < end {
		return nil
	}
	field, _ := dialogue.Field("end")
	rule := Rules[IssueEventDuration]
	detail := "Start and End are equal, so the event has zero duration and never renders."
	if start > end {
		detail = "End is earlier than Start; libass keeps an event that never matches any frame and VSFilter drops the entry while parsing."
	}
	return []Diagnostic{{
		ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
		Fix: rule.Fix, Line: dialogue.Line, Column: field.Start - dialogue.LineStart + 1, Field: "End",
		Detail: detail, Sources: rule.Sources,
	}}
}

func analyzeLayer(dialogue ass.Dialogue) []Diagnostic {
	field, ok := dialogue.Field("layer")
	if !ok {
		return nil
	}
	value := strings.TrimSpace(field.Value)
	if eventInteger(value) {
		return nil
	}
	rule := Rules[IssueEventLayer]
	detail := fmt.Sprintf("The Layer value %q has no leading integer; libass silently uses layer 0 while VSFilter rejects the line.", value)
	if value == "" {
		detail = "The Layer field is empty; libass silently uses layer 0 while VSFilter rejects the line."
	}
	return []Diagnostic{{
		ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
		Fix: rule.Fix, Line: dialogue.Line, Column: field.Start - dialogue.LineStart + 1, Field: "Layer",
		Detail: detail, Sources: rule.Sources,
	}}
}

// analyzeKaraoke walks the karaoke cursor (syllable timeline) the way both
// engines do and warns when it runs past the event duration, leaving trailing
// syllables stuck un-highlighted. It only models the shared cursor: every
// \k/\K/\kf/\ko advances the timeline by its argument in centiseconds (default
// 100 cs = 1000 ms). Word boundaries advance the sweep start but not the total,
// so they do not change the sum. \kt is a v4++ absolute reset whose exact
// semantics differ between engines, and a VSFilterMod-only tag can rewrite
// timing, so both cases bail to avoid a false claim.
func analyzeKaraoke(dialogue ass.Dialogue) []Diagnostic {
	return analyzeKaraokeWithCursor(dialogue, func() (int64, bool) { return karaokeCursor(dialogue.ParsedText()) })
}

func analyzeKaraokeForRenderer(dialogue ass.Dialogue, profile renderer.Profile) []Diagnostic {
	return analyzeKaraokeWithCursor(dialogue, func() (int64, bool) {
		return karaokeCursorResolved(ass.ParseConcreteDialogue(dialogue.Text), profile)
	})
}

func analyzeKaraokeWithCursor(dialogue ass.Dialogue, findCursor func() (int64, bool)) []Diagnostic {
	start, startOK := dialogueTime(dialogue, "start")
	end, endOK := dialogueTime(dialogue, "end")
	if !startOK || !endOK {
		return nil
	}
	duration := end - start
	if duration <= 0 {
		return nil
	}
	cursor, modeled := findCursor()
	if !modeled {
		return nil
	}
	if cursor <= duration {
		return nil
	}
	rule := Rules[IssueKaraoke]
	field, _ := dialogue.Field("end")
	return []Diagnostic{{
		ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
		Fix: rule.Fix, Line: dialogue.Line, Column: field.Start - dialogue.LineStart + 1, Field: "Text",
		Detail:  fmt.Sprintf("The karaoke syllables total %d ms, longer than the %d ms event, so the trailing syllables never reach their highlight window.", cursor, duration),
		Sources: rule.Sources,
	}}
}

// karaokeCursor sums the karaoke timeline in milliseconds. modeled is false
// when a construct outside the shared cursor model (\kt, a syllable inside a
// \t transition, or a non-numeric syllable) makes the total engine-specific.
func karaokeCursor(tree ass.DialogueText) (int64, bool) {
	const defaultSyllable = 1000
	var cursor int64
	sawKaraoke := false
	modeled := true
	tree.WalkTokens(func(token ass.TokenView) bool {
		if !token.HasTag {
			return true
		}
		tag := token.Tag
		switch tag.Name {
		case "kt":
			modeled = false
			return false
		case "k", "K", "kf", "ko":
			if tag.InTransition {
				modeled = false
				return false
			}
			sawKaraoke = true
			duration := int64(defaultSyllable)
			switch len(tag.Args) {
			case 0:
			case 1:
				value := ass.DecodeNumber(tag.Args[0])
				if value.Status != ass.ValueValid || value.Number < 0 || value.Number > float64(int64(^uint64(0)>>1)/10) {
					modeled = false
					return false
				}
				duration = int64(value.Number) * 10
			default:
				modeled = false
				return false
			}
			cursor += duration
		}
		return true
	})
	if !modeled || !sawKaraoke {
		return 0, false
	}
	return cursor, true
}

// eventInteger mirrors what both engines accept for an integer cell: decimal
// with optional sign, or &h/0x hexadecimal; libass mystrtou32_modulo silently
// stops at the first invalid character (ass.c:292-337), so require the whole
// cell to consume.
func eventInteger(value string) bool {
	lower := strings.ToLower(value)
	base := 10
	if strings.HasPrefix(lower, "&h") || strings.HasPrefix(lower, "0x") {
		value, base = value[2:], 16
	}
	if base == 16 {
		_, err := strconv.ParseUint(value, 16, 32)
		return err == nil && value != ""
	}
	if strings.HasPrefix(value, "-") {
		_, err := strconv.ParseInt(value, 10, 32)
		return err == nil
	}
	_, err := strconv.ParseUint(value, 10, 32)
	return err == nil
}

func analyzeEffect(dialogue ass.Dialogue) []Diagnostic {
	field, ok := dialogue.Field("effect")
	if !ok {
		return nil
	}
	value := strings.TrimSpace(field.Value)
	if value == "" {
		return nil
	}
	effect, params := splitEffect(value)
	canonical, recognized := knownEffect(effect)
	if !recognized {
		rule := Rules[IssueEffectField]
		return []Diagnostic{{
			ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
			Fix: rule.Fix, Line: dialogue.Line, Column: field.Start - dialogue.LineStart + 1, Field: "Effect",
			Detail:  fmt.Sprintf("Effect %q is not one of Banner;, Scroll up;, Scroll down;; every renderer treats the whole field as a no-op.", value),
			Sources: rule.Sources,
		}}
	}
	rule := Rules[IssueEffectField]
	if effect != canonical {
		return []Diagnostic{{
			ID: rule.ID, Severity: Warning, Title: rule.Title, Description: rule.Description,
			Fix: rule.Fix, Line: dialogue.Line, Column: field.Start - dialogue.LineStart + 1, Field: "Effect",
			Detail:  fmt.Sprintf("libass only accepts the exact spelling %q; %q runs in VSFilter but is ignored by libass.", canonical, effect),
			Sources: rule.Sources,
		}}
	}
	required := 1
	if canonical != "Banner;" {
		required = 3
	}
	if effectParamCount(params) < required {
		return []Diagnostic{{
			ID: rule.ID, Severity: rule.Severity, Title: rule.Title, Description: rule.Description,
			Fix: rule.Fix, Line: dialogue.Line, Column: field.Start - dialogue.LineStart + 1, Field: "Effect",
			Detail:  fmt.Sprintf("%q needs at least %d parameter(s) after it; without them the effect is a silent no-op.", canonical, required),
			Sources: rule.Sources,
		}}
	}
	return nil
}

// knownEffect reports the canonical spelling libass matches with a
// case-sensitive strncmp (ass_parse.c:940,975-977). VSFilter forks compare
// case-insensitively (xy-VSFilter RTS.cpp:1957,1972; VSFilterMod
// RTS.cpp:2130,2143), so a different case is a renderer divergence.
func knownEffect(effect string) (string, bool) {
	switch strings.ToLower(effect) {
	case "banner;":
		return "Banner;", true
	case "scroll up;":
		return "Scroll up;", true
	case "scroll down;":
		return "Scroll down;", true
	}
	return "", false
}

func splitEffect(value string) (effect, params string) {
	semi := strings.IndexByte(value, ';')
	if semi < 0 {
		return value, ""
	}
	return value[:semi+1], value[semi+1:]
}

func effectParamCount(params string) int {
	count := 0
	for _, part := range strings.Split(params, ";") {
		if eventInteger(strings.TrimSpace(part)) {
			count++
		}
	}
	return count
}

// dialogueTime parses a Start/End cell exactly like libass string2timecode:
// H:MM:SS.CS with the fourth segment counted as centiseconds. ok is false
// when the cell lacks the four-group shape both engines honor.
func dialogueTime(dialogue ass.Dialogue, name string) (int64, bool) {
	field, ok := dialogue.Field(name)
	if !ok {
		return 0, false
	}
	value := strings.TrimSpace(field.Value)
	if !eventTimeShape.MatchString(value) {
		return 0, false
	}
	groups := strings.FieldsFunc(value, func(r rune) bool { return r == ':' || r == '.' })
	hours, err := strconv.ParseInt(groups[0], 10, 64)
	if err != nil {
		return 0, false
	}
	minutes, err := strconv.ParseInt(groups[1], 10, 64)
	if err != nil {
		return 0, false
	}
	seconds, err := strconv.ParseInt(groups[2], 10, 64)
	if err != nil {
		return 0, false
	}
	cs, err := strconv.ParseInt(groups[3], 10, 64)
	if err != nil {
		return 0, false
	}
	return ((hours*60+minutes)*60+seconds)*1000 + cs*10, true
}
