package ass

import (
	"fmt"
	"regexp"
	"strconv"
)

type DrawingFragment struct {
	Text  string
	Start int
	End   int
}

type DrawingCoordinate struct {
	Raw   string
	Value float64
	Start int
	End   int
}

type DrawingCommand struct {
	Name        byte
	Start       int
	End         int
	Coordinates []DrawingCoordinate
}

type DrawingIssue struct {
	Start  int
	End    int
	Detail string
}

type Drawing struct {
	Fragments []DrawingFragment
	Commands  []DrawingCommand
	Issues    []DrawingIssue
}

var drawingNumberPrefix = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?`)

func (tree DialogueText) Drawings() []Drawing {
	var drawings []Drawing
	var fragments []DrawingFragment
	drawing := false

	flush := func() {
		if len(fragments) == 0 {
			return
		}
		drawings = append(drawings, ParseDrawing(fragments))
		fragments = nil
	}

	tree.WalkTokens(func(token TokenView) bool {
		if token.HasTag {
			tag := token.Tag
			if tag.InTransition || tag.Name != "p" {
				return true
			}
			value, known := tag.IntegerArgument()
			if !known {
				return true
			}
			next := value > 0
			if drawing && !next {
				flush()
			}
			drawing = next
			return true
		}
		if drawing && token.Text != "" {
			fragments = append(fragments, DrawingFragment{
				Text: token.Text, Start: token.Start, End: token.Start + len(token.Text),
			})
		}
		return true
	})
	if drawing {
		flush()
	}
	return drawings
}

func ParseDrawing(fragments []DrawingFragment) Drawing {
	drawing := Drawing{Fragments: append([]DrawingFragment(nil), fragments...)}
	var current *DrawingCommand

	finish := func() {
		if current == nil {
			return
		}
		validateDrawingCommand(&drawing, *current)
		drawing.Commands = append(drawing.Commands, *current)
		current = nil
	}

	for _, fragment := range fragments {
		for i := 0; i < len(fragment.Text); {
			ch := fragment.Text[i]
			if isSpace(ch) {
				i++
				continue
			}
			if isDrawingCommand(ch) {
				finish()
				current = &DrawingCommand{
					Name: ch, Start: fragment.Start + i, End: fragment.Start + i + 1,
				}
				i++
				continue
			}
			if match := drawingNumberPrefix.FindString(fragment.Text[i:]); match != "" {
				end := i + len(match)
				value, err := strconv.ParseFloat(match, 64)
				if err != nil {
					drawing.Issues = append(drawing.Issues, DrawingIssue{
						Start: fragment.Start + i, End: fragment.Start + end,
						Detail: fmt.Sprintf("Invalid drawing coordinate %q.", match),
					})
					i = end
					continue
				}
				coordinate := DrawingCoordinate{
					Raw: match, Value: value, Start: fragment.Start + i, End: fragment.Start + end,
				}
				if current == nil {
					drawing.Issues = append(drawing.Issues, DrawingIssue{
						Start: coordinate.Start, End: coordinate.End,
						Detail: "Drawing coordinate appears before any drawing command.",
					})
				} else {
					current.Coordinates = append(current.Coordinates, coordinate)
					current.End = coordinate.End
				}
				i = end
				continue
			}

			start := i
			for i < len(fragment.Text) && !isSpace(fragment.Text[i]) &&
				!isDrawingCommand(fragment.Text[i]) &&
				drawingNumberPrefix.FindString(fragment.Text[i:]) == "" {
				i++
			}
			if i == start {
				i++
			}
			drawing.Issues = append(drawing.Issues, DrawingIssue{
				Start: fragment.Start + start, End: fragment.Start + i,
				Detail: fmt.Sprintf("Unknown drawing data %q.", fragment.Text[start:i]),
			})
		}
	}
	finish()
	return drawing
}

func validateDrawingCommand(drawing *Drawing, command DrawingCommand) {
	count := len(command.Coordinates)
	valid := false
	switch command.Name {
	case 'm', 'n', 'l', 'p':
		valid = count >= 2 && count%2 == 0
	case 'b':
		valid = count >= 6 && count%6 == 0
	case 's':
		valid = count >= 6 && count%2 == 0
	case 'c':
		valid = count == 0
	}
	if valid {
		return
	}

	drawing.Issues = append(drawing.Issues, DrawingIssue{
		Start: command.Start, End: max(command.End, command.Start+1),
		Detail: fmt.Sprintf("Drawing command %q has %d coordinate value(s) with an invalid arity.", string(command.Name), count),
	})
}

func isDrawingCommand(ch byte) bool {
	switch ch {
	case 'm', 'n', 'l', 'b', 's', 'p', 'c':
		return true
	default:
		return false
	}
}
