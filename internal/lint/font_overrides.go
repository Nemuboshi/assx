package lint

import (
	"math"
	"sort"
	"strconv"
	"strings"

	"assx/internal/ass"
)

const (
	fontNameProperty = "fontname"
	fontSizeProperty = "fontsize"
	boldProperty     = "bold"
	italicProperty   = "italic"
	scaleXProperty   = "scalex"
	scaleYProperty   = "scaley"
	spacingProperty  = "spacing"
)

type styleRowKey struct {
	name   string
	folded string
	line   int
}

type fontBlockRange struct {
	start int
	end   int
}

// AnalyzeRedundantFontOverrides checks font state against each dialogue's style.
func AnalyzeRedundantFontOverrides(doc ass.Document) []Diagnostic {
	styles := fontStylesByName(doc.StyleFields)
	var diagnostics []Diagnostic
	for _, dialogue := range doc.Dialogues {
		fields, ok := styles[strings.TrimSpace(dialogue.Style)]
		if !ok {
			continue
		}
		diagnostic, ok := analyzeRedundantFontDialogue(dialogue, fields)
		if ok {
			diagnostics = append(diagnostics, diagnostic)
		}
	}
	return diagnostics
}

func fontStylesByName(fields []ass.StyleField) map[string]map[string]string {
	rows := make(map[styleRowKey]map[string]string)
	duplicates := make(map[styleRowKey]bool)
	for _, field := range fields {
		name := strings.TrimSpace(field.StyleName)
		fieldName := strings.ToLower(strings.TrimSpace(field.Name))
		if name == "" || fieldName == "" {
			continue
		}
		key := styleRowKey{name: name, folded: strings.ToLower(name), line: field.Line}
		row := rows[key]
		if row == nil {
			row = make(map[string]string)
			rows[key] = row
		}
		if _, exists := row[fieldName]; exists {
			duplicates[key] = true
		}
		row[fieldName] = field.Value
	}

	byFoldedName := make(map[string][]styleRowKey)
	for key := range rows {
		byFoldedName[key.folded] = append(byFoldedName[key.folded], key)
	}
	styles := make(map[string]map[string]string)
	for _, keys := range byFoldedName {
		if len(keys) != 1 || duplicates[keys[0]] {
			continue
		}
		styles[keys[0].name] = rows[keys[0]]
	}
	return styles
}

func analyzeRedundantFontDialogue(dialogue ass.Dialogue, styleFields map[string]string) (Diagnostic, bool) {
	style := make(map[string]string)
	for _, field := range []string{fontNameProperty, fontSizeProperty, boldProperty, italicProperty, scaleXProperty, scaleYProperty, spacingProperty} {
		if value, ok := canonicalStyleFontValue(field, styleFields[field]); ok {
			style[field] = value
		}
	}
	state := make(map[string]string, len(style))
	for property, value := range style {
		state[property] = value
	}

	tokens := Lex(dialogue.Text)
	var allTags, fontTags []Tag
	properties := make(map[string]bool)
	textStarted := false
	hasText := false
	for _, token := range tokens {
		if token.Tag == nil {
			if token.Text != "" {
				if !textStarted {
					for property := range properties {
						if state[property] != style[property] {
							return Diagnostic{}, false
						}
					}
					textStarted = true
				}
				hasText = true
			}
			continue
		}

		tag := *token.Tag
		allTags = append(allTags, tag)
		if tag.InTransition || tag.RepeatedSlashes > 0 || !safeIndependentFontTag(tag) {
			return Diagnostic{}, false
		}
		property, value, isFontTag, ok := fontAssignment(tag, style)
		if !ok {
			return Diagnostic{}, false
		}
		if !isFontTag {
			continue
		}
		_, exists := style[property]
		if !exists {
			return Diagnostic{}, false
		}
		properties[property] = true
		if textStarted && state[property] != value {
			return Diagnostic{}, false
		}
		state[property] = value
		fontTags = append(fontTags, tag)
	}
	if !hasText || len(fontTags) == 0 {
		return Diagnostic{}, false
	}
	for property := range properties {
		if state[property] != style[property] {
			return Diagnostic{}, false
		}
	}

	edits := redundantFontEdits(dialogue, allTags, fontTags)
	if len(edits) == 0 {
		return Diagnostic{}, false
	}
	first := fontTags[0]
	return Diagnostic{
		ID: IssueRedundantFontOverrides, Severity: Suggestion, FixSafety: SafeFix,
		Line: dialogue.Line, Column: first.Column, Tag: first.Name,
		Detail: "The font override tags leave the style font state unchanged across dialogue text.",
		Edits:  edits,
	}, true
}

func canonicalStyleFontValue(property, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	switch property {
	case fontNameProperty:
		return raw, raw != ""
	case boldProperty, italicProperty:
		value, err := strconv.Atoi(raw)
		if err != nil || (value != -1 && value != 0 && value != 1) {
			return "", false
		}
		if value == 0 {
			return "0", true
		}
		return "1", true
	case fontSizeProperty, scaleXProperty, scaleYProperty, spacingProperty:
		return canonicalFontNumber(raw)
	default:
		return "", false
	}
}

func canonicalFontNumber(raw string) (string, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return "", false
	}
	value32 := float32(value)
	if math.IsInf(float64(value32), 0) {
		return "", false
	}
	if value == 0 {
		value = 0
	}
	if value32 == 0 {
		value32 = 0
	}
	return strconv.FormatFloat(value, 'g', -1, 64) + ":" + strconv.FormatFloat(float64(value32), 'g', -1, 32), true
}

func fontAssignment(tag Tag, style map[string]string) (property, value string, isFontTag, ok bool) {
	name := strings.ToLower(tag.Name)
	switch name {
	case "fn":
		property = fontNameProperty
		if len(tag.Args) == 0 || (!tag.Paren && len(tag.Args) == 1 && strings.TrimSpace(tag.Args[0]) == "0") {
			value, ok = style[property]
			return property, value, true, ok
		}
		if tag.Paren || len(tag.Args) != 1 {
			return "", "", true, false
		}
		value = strings.TrimSpace(tag.Args[0])
		return property, value, true, value != ""
	case "fs":
		property = fontSizeProperty
		if len(tag.Args) == 0 {
			value, ok = style[property]
			return property, value, true, ok
		}
		if tag.Paren || len(tag.Args) != 1 {
			return "", "", true, false
		}
		raw := strings.TrimSpace(tag.Args[0])
		if strings.HasPrefix(raw, "+") || strings.HasPrefix(raw, "-") {
			return "", "", true, false
		}
		number, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
			return "", "", true, false
		}
		if number <= 0 {
			value, ok = style[property]
			return property, value, true, ok
		}
		value, ok = canonicalFontNumber(raw)
		return property, value, true, ok
	case "fscx", "fscy", "fsp":
		property = scaleXProperty
		if name == "fscy" {
			property = scaleYProperty
		} else if name == "fsp" {
			property = spacingProperty
		}
		if len(tag.Args) == 0 {
			value, ok = style[property]
			return property, value, true, ok
		}
		if tag.Paren || len(tag.Args) != 1 {
			return "", "", true, false
		}
		value, ok = canonicalFontTagNumber(tag.Args[0], property == scaleXProperty || property == scaleYProperty)
		return property, value, true, ok
	case "b", "i":
		property = boldProperty
		if name == "i" {
			property = italicProperty
		}
		if len(tag.Args) == 0 {
			value, ok = style[property]
			return property, value, true, ok
		}
		if tag.Paren || len(tag.Args) != 1 {
			return "", "", true, false
		}
		parsed, err := strconv.Atoi(strings.TrimSpace(tag.Args[0]))
		if err != nil || (parsed != 0 && parsed != 1) {
			return "", "", true, false
		}
		return property, strconv.Itoa(parsed), true, true
	default:
		return "", "", false, true
	}
}

func canonicalFontTagNumber(raw string, clampNegative bool) (string, bool) {
	raw = strings.TrimSpace(raw)
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return "", false
	}
	if clampNegative && value < 0 {
		return canonicalFontNumber("0")
	}
	return canonicalFontNumber(raw)
}

func safeIndependentFontTag(tag Tag) bool {
	name := strings.ToLower(tag.Name)
	switch name {
	case "r", "t", "p", "fe", "fsc", "fr", "frx", "fry", "frz", "fax", "fay", "n", "h":
		return false
	}
	spec, ok := TagSpecs[tag.Name]
	if !ok || spec.VSFilterModOnly || spec.Behavior == StyleReset || spec.Behavior == Transition {
		return false
	}
	if spec.Counts != nil {
		validCount := false
		for _, count := range spec.Counts {
			if count == len(tag.Args) {
				validCount = true
				break
			}
		}
		if !validCount {
			return false
		}
	}
	return true
}

func redundantFontEdits(dialogue ass.Dialogue, allTags, fontTags []Tag) []TextEdit {
	text := dialogue.Text
	candidate := make(map[[2]int]bool, len(fontTags))
	allByBlock := make(map[fontBlockRange][]Tag)
	fontByBlock := make(map[fontBlockRange][]Tag)
	for _, tag := range allTags {
		block, ok := fontTagBlock(text, tag)
		if !ok {
			continue
		}
		allByBlock[block] = append(allByBlock[block], tag)
	}
	for _, tag := range fontTags {
		candidate[[2]int{tag.Start, tag.End}] = true
		block, ok := fontTagBlock(text, tag)
		if ok {
			fontByBlock[block] = append(fontByBlock[block], tag)
		}
	}

	var edits []TextEdit
	for block, tags := range fontByBlock {
		all := allByBlock[block]
		if len(all) == len(tags) && onlyWhitespaceBetweenTags(text, block, all, candidate) {
			edits = append(edits, TextEdit{Start: dialogue.TextStart + block.start, End: dialogue.TextStart + block.end})
			continue
		}
		for _, tag := range tags {
			edits = append(edits, TextEdit{Start: dialogue.TextStart + tag.Start, End: dialogue.TextStart + tag.End})
		}
	}
	sort.Slice(edits, func(i, j int) bool { return edits[i].Start < edits[j].Start })
	return edits
}

func fontTagBlock(text string, tag Tag) (fontBlockRange, bool) {
	if tag.Start < 0 || tag.End > len(text) || tag.Start >= tag.End {
		return fontBlockRange{}, false
	}
	open := strings.LastIndex(text[:tag.Start], "{")
	closeOffset := strings.IndexByte(text[tag.End:], '}')
	if open < 0 || closeOffset < 0 {
		return fontBlockRange{}, false
	}
	close := tag.End + closeOffset
	if open >= tag.Start || close < tag.End {
		return fontBlockRange{}, false
	}
	return fontBlockRange{start: open, end: close + 1}, true
}

func onlyWhitespaceBetweenTags(text string, block fontBlockRange, tags []Tag, candidates map[[2]int]bool) bool {
	sort.Slice(tags, func(i, j int) bool { return tags[i].Start < tags[j].Start })
	cursor := block.start + 1
	for _, tag := range tags {
		if !candidates[[2]int{tag.Start, tag.End}] || tag.Start < cursor || tag.End > block.end-1 {
			return false
		}
		if strings.TrimSpace(text[cursor:tag.Start]) != "" {
			return false
		}
		cursor = tag.End
	}
	return strings.TrimSpace(text[cursor:block.end-1]) == ""
}
