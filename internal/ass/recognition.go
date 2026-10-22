package ass

import "strings"

// Classification describes whether the input has credible ASS/SSA structure.
type Classification string

const (
	Recognized Classification = "recognized"
	Tentative  Classification = "tentative"
	Rejected   Classification = "rejected"
)

// RecognitionEvidence identifies a structural clue and its physical line.
type RecognitionEvidence struct {
	ID   string
	Line int
}

type Recognition struct {
	Class    Classification
	Evidence []RecognitionEvidence
}

// Recognize checks content in its section context. A filename is not evidence.
// This is an input boundary, not ASS syntax or renderer validation.
func Recognize(text string) Recognition {
	tree := ParseConcreteDocument(text)
	result := Recognition{Class: Rejected}
	credible := false
	tentative := false
	foreign := false
	for _, line := range tree.Lines {
		for _, char := range line.Content {
			if (char < 0x20 && char != '\t') || char == 0x7f {
				return Recognition{Class: Rejected, Evidence: []RecognitionEvidence{{ID: "input.binary", Line: line.Number}}}
			}
		}
		if strings.EqualFold(strings.TrimSpace(line.Content), "WEBVTT") {
			foreign = true
			result.Evidence = append(result.Evidence, RecognitionEvidence{ID: "input.webvtt", Line: line.Number})
		} else if strings.Contains(line.Content, "-->") && strings.Contains(line.Content, ":") {
			foreign = true
			result.Evidence = append(result.Evidence, RecognitionEvidence{ID: "input.foreign-timing", Line: line.Number})
		}
		if line.Header != nil {
			section := strings.ToLower(strings.TrimSpace(line.Header.RawName))
			if section == "script info" || section == "events" || isStyleSection(section) {
				tentative = true
				id := "section." + section
				if section == "script info" {
					id = "section.script-info"
				}
				result.Evidence = append(result.Evidence, RecognitionEvidence{ID: id, Line: line.Number})
			}
			continue
		}
		if line.Record == nil {
			continue
		}
		section := strings.ToLower(strings.TrimSpace(line.Section))
		key := strings.ToLower(strings.TrimSpace(line.Record.Key))
		id := ""
		switch {
		case section == "events" && key == "dialogue":
			id = "events.dialogue"
		case section == "events" && key == "format" && meaningfulFormat(line.Record.Body, "start", "end", "text"):
			id = "events.format"
		case isStyleSection(section) && key == "style" && strings.TrimSpace(line.Record.Body) != "":
			id = "styles.style"
		case isStyleSection(section) && key == "format" && meaningfulFormat(line.Record.Body, "name", "fontname", "fontsize"):
			id = "styles.format"
		case section == "script info" && scriptHeader(key):
			id = "script." + key
		}
		if id != "" {
			credible = true
			result.Evidence = append(result.Evidence, RecognitionEvidence{ID: id, Line: line.Number})
		}
	}
	if credible {
		result.Class = Recognized
	} else if foreign {
		result.Class = Rejected
	} else if tentative {
		result.Class = Tentative
	}
	if len(result.Evidence) == 0 {
		result.Evidence = []RecognitionEvidence{{ID: "input.no-ass-structure"}}
	}
	return result
}

func meaningfulFormat(body string, names ...string) bool {
	found := make(map[string]bool, len(names))
	for _, field := range ParseFormat(body) {
		for _, name := range names {
			if field == name {
				found[name] = true
				break
			}
		}
	}
	return len(found) >= 2
}

func scriptHeader(key string) bool {
	switch key {
	case "scripttype", "playresx", "playresy", "layoutresx", "layoutresy", "ycbcr matrix", "wrapstyle", "scaledborderandshadow", "collisions", "timer":
		return true
	}
	return false
}
