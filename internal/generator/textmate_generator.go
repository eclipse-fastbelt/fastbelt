// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package generator

import (
	"encoding/json"
	"fmt"
	"strings"

	"typefox.dev/fastbelt/internal/grammar"
)

type TextMateGrammar struct {
	Repository        Repository         "json:\"repository\""
	ScopeName         string             "json:\"scopeName\""
	Patterns          []Pattern          "json:\"patterns\""
	Injections        map[string]Pattern "json:\"injections,omitempty\""
	InjectionSelector string             "json:\"injectionSelector,omitempty\""
	FileTypes         []string           "json:\"fileTypes,omitempty\""
	Name              string             "json:\"name,omitempty\""
	FirstLineMatch    string             "json:\"firstLineMatch,omitempty\""
}

type Repository = map[string]Pattern

type Pattern struct {
	Id                  int        "json:\"id,omitempty\""
	Include             string     "json:\"include,omitempty\""
	Name                string     "json:\"name,omitempty\""
	ContentName         string     "json:\"contentName,omitempty\""
	Match               string     "json:\"match,omitempty\""
	Captures            Captures   "json:\"captures,omitempty\""
	Begin               string     "json:\"begin,omitempty\""
	BeginCaptures       Captures   "json:\"beginCaptures,omitempty\""
	End                 string     "json:\"end,omitempty\""
	EndCaptures         Captures   "json:\"endCaptures,omitempty\""
	While               string     "json:\"while,omitempty\""
	WhileCaptures       Captures   "json:\"whileCaptures,omitempty\""
	Patterns            []Pattern  "json:\"patterns,omitempty\""
	Repository          Repository "json:\"repository,omitempty\""
	ApplyEndPatternLast bool       "json:\"applyEndPatternLast,omitempty\""
}

type Captures = map[string]Pattern

type TextMateGeneratorConfig struct {
	Id             string
	FileExtensions []string
}

func GenerateTextMate(grammar grammar.Grammar, config TextMateGeneratorConfig) string {
	obj := TextMateGrammar{
		Name:       string(config.Id),
		ScopeName:  fmt.Sprintf("source.%s", config.Id),
		FileTypes:  []string(config.FileExtensions),
		Patterns:   GetPatterns(grammar, config),
		Repository: GetRepository(grammar, config),
	}
	result, _ := json.MarshalIndent(obj, "", "  ")
	return string(result)
}

func GetPatterns(grammar grammar.Grammar, config TextMateGeneratorConfig) []Pattern {
	patterns := []Pattern{}
	patterns = append(patterns, Pattern{
		Include: "#comments",
	})
	patterns = append(patterns, GetControlKeywords(grammar, config))
	patterns = append(patterns, GetStringPatterns(grammar, config)...)

	return patterns
}

func GetRepository(grammar grammar.Grammar, config TextMateGeneratorConfig) Repository {
	repository := Repository{}
	commentPatterns := []Pattern{}
	var stringEscapePattern *Pattern
	for _, terminal := range grammar.Terminals() {
		if regexPattern := tokenRegexp(terminal); terminal.Modifier() == "comment" && regexPattern != "" {
			// A regex that cannot be split simply yields no comment patterns.
			parts, _ := SplitDelimiters(regexPattern[1 : len(regexPattern)-1])
			for _, part := range parts {
				if part.Begin == "" {
					continue
				}
				if part.End != "" {
					commentPatterns = append(commentPatterns, Pattern{
						Name:  fmt.Sprintf(`comment.block.%s`, config.Id),
						Begin: part.Begin,
						BeginCaptures: Captures{
							"0": Pattern{
								Name: fmt.Sprintf(`punctuation.definition.comment.%s`, config.Id),
							},
						},
						End: part.End,
						EndCaptures: Captures{
							"0": Pattern{
								Name: fmt.Sprintf(`punctuation.definition.comment.%s`, config.Id),
							},
						},
					})
				} else {
					commentPatterns = append(commentPatterns, Pattern{
						Begin: part.Begin,
						BeginCaptures: Captures{
							"1": Pattern{
								Name: fmt.Sprintf(`punctuation.whitespace.comment.leading.%s`, config.Id),
							},
						},
						End:  `(?=$)`,
						Name: fmt.Sprintf(`comment.line.%s`, config.Id),
					})
				}
			}
		} else if strings.ToLower(terminal.Name()) == "string" {
			stringEscapePattern = &Pattern{
				Name:  fmt.Sprintf(`constant.character.escape.%s`, config.Id),
				Match: `\\(x[0-9A-Fa-f]{2}|u[0-9A-Fa-f]{4}|u\{[0-9A-Fa-f]+\}|[0-2][0-7]{0,2}|3[0-6][0-7]?|37[0-7]?|[4-7][0-7]?|.|$)`,
			}
		}

		if len(commentPatterns) > 0 {
			repository["comments"] = Pattern{
				Patterns: commentPatterns,
			}
		}
		if stringEscapePattern != nil {
			repository["string-character-escape"] = *stringEscapePattern
		}
	}
	return repository
}

func GetControlKeywords(grammr grammar.Grammar, config TextMateGeneratorConfig) Pattern {
	controlKeywords := []string{}
	for _, keyword := range GetAllKeywords(grammr).Keywords {
		if value := grammar.KeywordValue(keyword); keywordScope(value) == "keyword.control" {
			controlKeywords = append(controlKeywords, value)
		}
	}
	return Pattern{
		Name:  fmt.Sprintf(`keyword.control.%s`, config.Id),
		Match: keywordsPattern(controlKeywords),
	}
}

func GetStringPatterns(grammr grammar.Grammar, config TextMateGeneratorConfig) []Pattern {
	terminals := grammr.Terminals()
	var stringTerminal grammar.TokenDecl
	for _, terminal := range terminals {
		if strings.ToLower(terminal.Name()) == "string" {
			stringTerminal = terminal
			break
		}
	}
	stringPatterns := []Pattern{}
	if regexPattern := tokenRegexp(stringTerminal); regexPattern != "" {
		// A regex that cannot be split simply yields no string patterns.
		parts, _ := SplitDelimiters(grammar.RegexpValue(regexPattern))
		for _, part := range parts {
			if part.Begin != "" && part.End != "" {
				stringPatterns = append(stringPatterns, Pattern{
					Name:  fmt.Sprintf(`string.quoted.%s.%s`, delimiterName(part.Begin), config.Id),
					Begin: part.Begin,
					End:   part.End,
					Patterns: []Pattern{
						{
							Include: "#string-character-escape",
						},
					},
				})
			}
		}
	}
	return stringPatterns
}

func delimiterName(delimiter string) string {
	switch delimiter {
	case "'":
		return "single"
	case `"`:
		return "double"
	case "`":
		return "backtick"
	default:
		return "delimiter"
	}
}

// tokenRegexp returns the regex literal (including slashes) of a regex token,
// or "" if t is nil or not a regex token.
func tokenRegexp(t grammar.TokenDecl) string {
	if t == nil {
		return ""
	}
	if content, ok := t.Content().(grammar.RegexpTokenContent); ok {
		return content.Regexp()
	}
	return ""
}
