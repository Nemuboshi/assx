package ass

import "testing"

func TestDrawingASTAndArity(t *testing.T) {
	drawing := ParseDrawing([]DrawingFragment{{Text: "m 0 0 l 10 10 b 0 0 5 5 10 10", Start: 5, End: 36}})
	if len(drawing.Issues) != 0 || len(drawing.Commands) != 3 || drawing.Commands[2].Name != 'b' || len(drawing.Commands[2].Coordinates) != 6 {
		t.Fatalf("drawing=%#v", drawing)
	}
}

func TestDrawingASTReportsMalformedCoordinates(t *testing.T) {
	drawing := ParseDrawing([]DrawingFragment{{Text: "m 0 0 l 10 b 0 0 5 5"}})
	if len(drawing.Issues) != 2 {
		t.Fatalf("drawing=%#v", drawing)
	}
}

func TestDrawingContinuesAcrossFragments(t *testing.T) {
	drawing := ParseDrawing([]DrawingFragment{{Text: "m 0 0", Start: 5, End: 10}, {Text: " l 10 10", Start: 25, End: 33}})
	if len(drawing.Commands) != 2 || len(drawing.Issues) != 0 || drawing.Commands[1].Coordinates[0].Start != 28 {
		t.Fatalf("drawing=%#v", drawing)
	}
}
