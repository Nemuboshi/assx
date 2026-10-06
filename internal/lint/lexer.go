package lint

import (
	"sort"
	"strings"
)

type Tag struct {
	Name            string   `json:"tag"`
	Args            []string `json:"arguments,omitempty"`
	Column          int      `json:"column"`
	Paren           bool     `json:"-"`
	Start           int      `json:"-"`
	End             int      `json:"-"`
	InTransition    bool     `json:"-"`
	RepeatedSlashes int      `json:"-"`
}

type Token struct {
	Text  string
	Tag   *Tag
	Start int
}

var tagNames []string

func init() {
	for name := range TagSpecs {
		tagNames = append(tagNames, name)
	}
	sort.Slice(tagNames, func(i, j int) bool {
		if len(tagNames[i]) == len(tagNames[j]) {
			return tagNames[i] < tagNames[j]
		}
		return len(tagNames[i]) > len(tagNames[j])
	})
}

func Lex(text string) []Token {
	var tokens []Token
	for offset := 0; offset < len(text); {
		open := strings.IndexByte(text[offset:], '{')
		if open < 0 {
			tokens = appendText(tokens, text[offset:], offset)
			break
		}
		open += offset
		if open > offset {
			tokens = appendText(tokens, text[offset:open], offset)
		}
		close := strings.IndexByte(text[open:], '}')
		if close < 0 {
			tokens = appendText(tokens, text[open:], open)
			break
		}
		close += open
		tokens = append(tokens, lexBlock(text[open+1:close], open+1, false)...)
		offset = close + 1
	}
	return tokens
}

func appendText(tokens []Token, text string, start int) []Token {
	if text != "" {
		return append(tokens, Token{Text: text, Start: start})
	}
	return tokens
}

func lexBlock(block string, base int, inTransition bool) []Token {
	var tokens []Token
	for i := 0; i < len(block); {
		backslash := strings.IndexByte(block[i:], '\\')
		if backslash < 0 {
			break
		}
		backslash += i
		if runEnd := repeatedSlashEnd(block, backslash); runEnd > backslash+1 {
			tokens = append(tokens, Token{Tag: &Tag{
				Column: base + backslash + 1, Start: base + backslash, End: base + runEnd - 1,
				RepeatedSlashes: runEnd - backslash - 1,
			}})
			i = runEnd - 1
			continue
		}
		j := backslash + 1
		for j < len(block) && isSpace(block[j]) {
			j++
		}
		k := j
		for k < len(block) && block[k] != '(' && block[k] != '\\' {
			k++
		}
		if k == j {
			i = j
			continue
		}
		head := strings.TrimSpace(block[j:k])
		name := ""
		for _, candidate := range tagNames {
			if strings.HasPrefix(head, candidate) && recognizedTagPrefix(candidate, head[len(candidate):]) {
				name = candidate
				break
			}
		}
		if name == "" {
			name = head
			if len(name) > 12 {
				name = name[:12]
			}
			tokens = append(tokens, Token{Tag: &Tag{Name: name, Column: base + j + 1, Start: base + backslash, End: base + k, InTransition: inTransition}})
			i = k
			continue
		}
		var args []string
		paren := k < len(block) && block[k] == '('
		if paren {
			k++
			for {
				for k < len(block) && isSpace(block[k]) {
					k++
				}
				start := k
				for k < len(block) && block[k] != ',' && block[k] != '\\' && block[k] != ')' {
					k++
				}
				if k < len(block) && block[k] == ',' {
					args = append(args, strings.TrimSpace(block[start:k]))
					k++
					continue
				}
				if k < len(block) && block[k] == '\\' {
					if close := strings.IndexByte(block[k:], ')'); close >= 0 {
						k += close
					} else {
						k = len(block)
					}
				}
				if k > start || len(args) > 0 {
					args = append(args, strings.TrimSpace(block[start:k]))
				}
				if k < len(block) && block[k] == ')' {
					k++
				}
				break
			}
		} else if rest := strings.TrimSpace(head[len(name):]); rest != "" {
			args = []string{rest}
		}
		tag := Tag{Name: name, Args: args, Column: base + j + 1, Paren: paren, InTransition: inTransition, Start: base + backslash, End: base + k}
		tokens = append(tokens, Token{Tag: &tag})
		if tag.Name == "t" && len(args) > 0 {
			argumentStart := strings.LastIndex(block[:k], args[len(args)-1])
			if argumentStart >= 0 {
				tokens = append(tokens, lexBlock(args[len(args)-1], base+argumentStart, true)...)
			}
		}
		i = k
	}
	return tokens
}

func recognizedTagPrefix(name, value string) bool {
	if value == "" || TagSpecs[name].Value == FontNameValue || TagSpecs[name].Behavior == StyleReset {
		return true
	}
	first := value[0]
	isLetter := (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')
	return !isLetter || len(name) > 1
}
func repeatedSlashEnd(block string, start int) int {
	end := start
	for end < len(block) && block[end] == '\\' {
		end++
	}
	return end
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\f' || b == '\v'
}
