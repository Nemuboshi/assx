package ass

import "testing"

func TestRecognizeContentInSectionContext(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		want       Classification
		id         string
	}{
		{"events dialogue without format", "[Events]\nDialogue: broken\n", Recognized, "events.dialogue"},
		{"repeated events", "[Events]\n[Unknown]\n[Events]\nDialogue: broken\n", Recognized, "events.dialogue"},
		{"events format", "[Events]\nFormat: Layer, Start, End, Style, Text\n", Recognized, "events.format"},
		{"style record", "[V4 Styles]\nStyle: Default,Arial,20\n", Recognized, "styles.style"},
		{"style format", "[V4+ Styles]\nFormat: Name, Fontname, Fontsize\n", Recognized, "styles.format"},
		{"script header", "[Script Info]\nPlayResX: wrong\n", Recognized, "script.playresx"},
		{"case variants", "[eVeNtS]\ndIaLoGuE: broken\n", Recognized, "events.dialogue"},
		{"empty events", "[Events]\n", Tentative, "section.events"},
		{"empty script", "[Script Info]\nTitle: Example\n", Tentative, "section.script-info"},
		{"empty", "", Rejected, ""},
		{"ordinary text", "This is a note.\n", Rejected, ""},
		{"srt", "1\n00:00:01,000 --> 00:00:02,000\nHello\n", Rejected, "input.foreign-timing"},
		{"webvtt", "WEBVTT\n\n00:01.000 --> 00:02.000\nHello\n", Rejected, "input.webvtt"},
		{"foreign dominates empty section", "[Events]\nWEBVTT\n00:01.000 --> 00:02.000\n", Rejected, "input.webvtt"},
		{"binary", "[Events]\nDialogue: x\x00y\n", Rejected, "input.binary"},
		{"control bytes", "[Events]\nDialogue: x\x01y\n", Rejected, "input.binary"},
		{"delete byte", "[Events]\nDialogue: x\x7fy\n", Rejected, "input.binary"},
		{"legacy bytes retain structural evidence", "[Events]\nDialogue: \xff\n", Recognized, "events.dialogue"},
		{"isolated dialogue", "Dialogue: 0,0,1,Default,text\n", Rejected, ""},
		{"unknown section format", "[Other]\nFormat: Name, Fontname, Fontsize\n", Rejected, ""},
		{"quoted section", "Here is [Events] and Dialogue: a line\n", Rejected, ""},
		{"weak format", "[Events]\nFormat: Something\n", Tentative, "section.events"},
		{"comment only may be generic INI", "[Events]\nComment: project notes\n", Tentative, "section.events"},
		{"repeated format name", "[Events]\nFormat: Start, Start\n", Tentative, "section.events"},
		{"credible ass outweighs foreign prose", "[Events]\nDialogue: broken\nWEBVTT\n", Recognized, "events.dialogue"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Recognize(tc.text)
			if got.Class != tc.want {
				t.Fatalf("classification = %s, want %s; evidence = %+v", got.Class, tc.want, got.Evidence)
			}
			if tc.id != "" {
				found := false
				for _, evidence := range got.Evidence {
					if evidence.ID == tc.id && evidence.Line > 0 {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing evidence %q: %+v", tc.id, got.Evidence)
				}
			}
		})
	}
}
