package ass

import "testing"

func TestDrawingASTAndArity(t *testing.T) {
	tree := ParseDialogueText(`{\p1}m 0 0 l 10 10 b 0 0 5 5 10 10{\p0}`)
	drawings := tree.Drawings()
	if len(drawings) != 1 || len(drawings[0].Issues) != 0 || len(drawings[0].Commands) != 3 {
		t.Fatalf("drawing = %#v", drawings)
	}
	if drawings[0].Commands[2].Name != 'b' || len(drawings[0].Commands[2].Coordinates) != 6 {
		t.Fatalf("bezier = %#v", drawings[0].Commands[2])
	}
}

func TestDrawingASTReportsMalformedCoordinates(t *testing.T) {
	tree := ParseDialogueText(`{\p1}m 0 0 l 10 b 0 0 5 5{\p0}`)
	drawings := tree.Drawings()
	if len(drawings) != 1 || len(drawings[0].Issues) != 2 {
		t.Fatalf("drawing issues = %#v", drawings)
	}
}

func TestDrawingStateContinuesAcrossOverrideBlocks(t *testing.T) {
	tree := ParseDialogueText(`{\p1}m 0 0{\c&HFFFFFF&} l 10 10{\p0}`)
	drawings := tree.Drawings()
	if len(drawings) != 1 || len(drawings[0].Commands) != 2 || len(drawings[0].Issues) != 0 {
		t.Fatalf("drawing = %#v", drawings)
	}
}
