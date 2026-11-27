// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

// tmGrammar is the JSON structure of a *.tmLanguage.json file. Only the
// fields used by the generator are declared.
type tmGrammar struct {
	Name       string       `json:"name,omitempty"`
	ScopeName  string       `json:"scopeName"`
	FileTypes  []string     `json:"fileTypes,omitempty"`
	Patterns   []tmPattern  `json:"patterns"`
	Repository tmRepository `json:"repository,omitempty"`
}

// tmRepository maps repository keys to rules.
type tmRepository = map[string]tmPattern

// tmPattern is a TextMate rule.
type tmPattern struct {
	Include       string      `json:"include,omitempty"`
	Name          string      `json:"name,omitempty"`
	Match         string      `json:"match,omitempty"`
	Captures      tmCaptures  `json:"captures,omitempty"`
	Begin         string      `json:"begin,omitempty"`
	BeginCaptures tmCaptures  `json:"beginCaptures,omitempty"`
	End           string      `json:"end,omitempty"`
	EndCaptures   tmCaptures  `json:"endCaptures,omitempty"`
	Patterns      []tmPattern `json:"patterns,omitempty"`
}

// tmCaptures maps capture group numbers to the rules applied to them.
type tmCaptures = map[string]tmPattern

// generateCommon generates the common TextMate grammar shared by all
// languages of a build, with scope name [CommonScopeName] of the grammar
// name. overrides maps token names to scopes (see textMateScopes). Token mode
// constructs that TextMate can only approximate are reported as warnings to
// artifacts.Logger.
//
// The repository has these entries:
//   - <EntryRule>: one per language; includes the language's entry mode
//   - mode_<Mode>: the rules active in a token mode, as includes
//   - keywords_<Mode>: one match rule for the keywords of a mode
//   - token_<Name>, token_<Name>_comment, token_<Name>_hidden: a token with
//     the given modifier
//   - token_<Name>[_<modifier>]_push_<Target>: a begin/end rule for a token
//     that pushes Target; it ends at Target's pop tokens
//   - token_<Name>[_<modifier>]_mode_<Target>_in_<Mode>: a begin/end rule for
//     a token that switches from Mode to Target; it ends before Mode's pop tokens
//
// Inside a mode, the rules are ordered: mode commands, comment, hidden and
// string tokens, keywords, other tokens. TextMate takes the leftmost match and
// breaks ties by rule order, while the lexer takes the longest match. Putting
// comments before keywords lets "//" win over a "/" keyword, and putting
// keywords before other tokens lets "if" win over an ID.
func generateCommon(artifacts pluginapi.BuildArtifacts, overrides map[string]string) (string, error) {
	g := &textMateGenerator{
		model:      buildTextMateModel(artifacts.TokenModes, artifacts.Languages),
		scopes:     textMateScopes{Grammar: strings.ToLower(artifacts.Grammar.Name), Overrides: overrides},
		repository: tmRepository{},
		logger:     artifacts.Logger,
	}
	for _, mode := range g.model.Modes {
		if err := g.generateMode(mode); err != nil {
			return "", err
		}
	}
	g.checkOverrides()
	for _, language := range g.model.Languages {
		g.repository[language.Entry] = tmPattern{Patterns: []tmPattern{include(modeKey(language.EntryMode))}}
	}
	return marshalTextMate(tmGrammar{
		ScopeName:  CommonScopeName(artifacts.Grammar.Name),
		Patterns:   []tmPattern{},
		Repository: g.repository,
	})
}

// checkOverrides warns about scope overrides for tokens that no mode uses.
func (g *textMateGenerator) checkOverrides() {
	names := map[string]bool{}
	for _, mode := range g.model.Modes {
		for _, token := range mode.Tokens {
			names[token.Name] = true
		}
	}
	var unknown []string
	for name := range g.scopes.Overrides {
		if !names[name] {
			unknown = append(unknown, name)
		}
	}
	slices.Sort(unknown)
	for _, name := range unknown {
		g.logger.Warn(fmt.Sprintf("TextMate: scope override for unknown token %s", name))
	}
}

// marshalTextMate renders a TextMate grammar as indented JSON. Map keys are
// sorted by encoding/json, so the output is deterministic.
func marshalTextMate(tm tmGrammar) (string, error) {
	buf := &bytes.Buffer{}
	encoder := json.NewEncoder(buf)
	// Regexes contain '&', '<' and '>', which would otherwise be escaped.
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(tm); err != nil {
		return "", err
	}
	return buf.String(), nil
}

type textMateGenerator struct {
	model      textMateModel
	scopes     textMateScopes
	repository tmRepository
	logger     *slog.Logger
}

func include(key string) tmPattern {
	return tmPattern{Include: "#" + key}
}

func modeKey(mode string) string {
	return "mode_" + mode
}

func tokenKey(token textMateToken) string {
	key := "token_" + token.Name
	if token.Modifier != tokens.ModifierNone {
		key += "_" + string(token.Modifier)
	}
	return key
}

func (g *textMateGenerator) generateMode(mode *textMateMode) error {
	var commands, blocks, others []tmPattern
	var keywords []scopedKeyword
	for _, token := range mode.Tokens {
		switch {
		case token.Keyword == "" && token.Regexp == "":
			continue
		case token.Command == tokens.CommandPop:
			// Pop tokens are the end of the rules that enter this mode.
			continue
		case token.Command != "":
			key, err := g.commandRule(mode, token)
			if err != nil {
				return err
			}
			commands = append(commands, include(key))
		case token.Keyword != "":
			keywords = append(keywords, scopedKeyword{Value: token.Keyword, Scope: g.scopes.scopeOf(token, delimiters{})})
		default:
			key, block, err := g.tokenRule(token)
			if err != nil {
				return err
			}
			switch {
			case key == "":
			case block:
				blocks = append(blocks, include(key))
			default:
				others = append(others, include(key))
			}
		}
	}
	var keywordRules []tmPattern
	if len(keywords) > 0 {
		key := "keywords_" + mode.Name
		g.repository[key] = keywordsMatch(keywords)
		keywordRules = append(keywordRules, include(key))
	}
	g.repository[modeKey(mode.Name)] = tmPattern{Patterns: slices.Concat(commands, blocks, keywordRules, others)}
	return nil
}

// tokenRule adds the rule for a regexp token without a mode command and
// returns its key, or "" if the token needs no rule. block reports whether the
// token is a comment, hidden or string token. Those are ordered before
// keywords and may span lines.
func (g *textMateGenerator) tokenRule(token textMateToken) (key string, block bool, err error) {
	scope := g.scopes.scopeOf(token, delimiters{})
	block = token.Modifier != tokens.ModifierNone || isStringScope(scope) || strings.HasPrefix(scope, "comment.")
	key = tokenKey(token)
	if _, ok := g.repository[key]; ok {
		return key, block, nil
	}
	re, err := syntax.Parse(token.Regexp, syntax.Perl)
	if err != nil {
		return "", false, fmt.Errorf("token %s: %w", token.Name, err)
	}
	if token.Modifier == tokens.ModifierHidden && scope == "" && onlyWhitespace(re) {
		// Nothing would be highlighted inside whitespace anyway.
		return "", false, nil
	}
	if block {
		rule, ok, err := g.beginEndRule(token, re)
		if err != nil {
			return "", false, err
		}
		if ok {
			g.repository[key] = rule
			return key, block, nil
		}
	}
	g.repository[key] = tmPattern{Match: onigurumaOf(re), Name: scope}
	return key, block, nil
}

func isStringScope(scope string) bool {
	return strings.HasPrefix(scope, "string.") || scope == "string"
}

// beginEndRule splits a comment, hidden or string token into begin/end rules, one per
// alternative. ok is false if the token is better emitted as a single match:
// when no alternative has an end delimiter (e.g. line comments), or when an
// alternative has no literal begin.
func (g *textMateGenerator) beginEndRule(token textMateToken, re *syntax.Regexp) (rule tmPattern, ok bool, err error) {
	parts, err := splitDelimiters(token.Regexp)
	if errors.Is(err, errTooManyAlternatives) {
		return tmPattern{}, false, nil
	} else if err != nil {
		return tmPattern{}, false, fmt.Errorf("token %s: %w", token.Name, err)
	}
	hasEnd := false
	for _, part := range parts {
		if part.Begin == "" || !part.HasBody {
			return tmPattern{}, false, nil
		}
		hasEnd = hasEnd || part.End != ""
	}
	if !hasEnd {
		return tmPattern{}, false, nil
	}
	var rules []tmPattern
	for _, part := range parts {
		scope := g.scopes.scopeOf(token, part)
		r := tmPattern{Name: scope, Begin: part.Begin, End: part.End}
		if r.End == "" {
			// TextMate cannot match the variable body, so the rule ends with the line.
			r.End = "$"
		}
		if isStringScope(scope) && hasBackslashLiteral(re) {
			r.Patterns = []tmPattern{{Match: `\\.`, Name: g.scopes.withSuffix("constant.character.escape")}}
		}
		rules = append(rules, r)
	}
	if len(rules) == 1 {
		return rules[0], true, nil
	}
	return tmPattern{Patterns: rules}, true, nil
}

// commandRule adds the begin/end rule for a token with a push or mode command
// in mode and returns its key.
func (g *textMateGenerator) commandRule(mode *textMateMode, token textMateToken) (string, error) {
	key := tokenKey(token) + "_" + string(token.Command) + "_" + token.Target
	if token.Command == tokens.CommandMode {
		key += "_in_" + mode.Name
	}
	if _, ok := g.repository[key]; ok {
		return key, nil
	}
	begin, err := tokenOniguruma(token)
	if err != nil {
		return "", err
	}
	target := g.model.ModesByName[token.Target]
	rule := tmPattern{Begin: begin, Patterns: []tmPattern{include(modeKey(target.Name))}}
	if scope := g.scopes.scopeOf(token, delimiters{}); scope != "" {
		rule.BeginCaptures = tmCaptures{"0": {Name: scope}}
	}
	switch token.Command {
	case tokens.CommandPush:
		rule.End, rule.EndCaptures, err = g.popPattern(target)
		if err != nil {
			return "", err
		}
	case tokens.CommandMode:
		// TextMate cannot replace the rule that entered mode. Instead the new
		// rule ends right before mode's end, which the enclosing rule matches.
		parentEnd, _, err := g.popPattern(mode)
		if err != nil {
			return "", err
		}
		if parentEnd == "(?!)" {
			rule.End = parentEnd
		} else {
			rule.End = "(?=" + parentEnd + ")"
		}
		if !slices.Equal(tokenIndices(mode.PopTokens()), tokenIndices(target.PopTokens())) {
			g.logger.Warn(fmt.Sprintf(
				"TextMate: %s switches from token mode %s to %s, which have different pop tokens; "+
					"highlighting of %s ends at the pop tokens of %s", token.Name, mode.Name, target.Name, target.Name, mode.Name))
		}
	}
	g.repository[key] = rule
	return key, nil
}

func tokenIndices(tokens []textMateToken) []int {
	result := make([]int, 0, len(tokens))
	for _, token := range tokens {
		result = append(result, token.TokenIndex)
	}
	slices.Sort(result)
	return result
}

// popPattern returns the end pattern for rules that enter mode: its pop
// tokens as alternatives, each in a capture group that gets the token's scope.
// A mode without pop tokens never ends: (?!).
func (g *textMateGenerator) popPattern(mode *textMateMode) (string, tmCaptures, error) {
	pops := mode.PopTokens()
	if len(pops) == 0 {
		return "(?!)", nil, nil
	}
	captures := tmCaptures{}
	alternatives := make([]string, 0, len(pops))
	group := 0
	if len(pops) > 1 {
		group = 1
	}
	for _, token := range pops {
		regex, err := tokenOniguruma(token)
		if err != nil {
			return "", nil, err
		}
		if scope := g.scopes.scopeOf(token, delimiters{}); scope != "" {
			captures[strconv.Itoa(group)] = tmPattern{Name: scope}
		}
		if len(pops) > 1 {
			regex = "(" + regex + ")"
			// Captures inside the token regex shift the following groups.
			group += 1 + captureCount(token)
		}
		alternatives = append(alternatives, regex)
	}
	if len(captures) == 0 {
		captures = nil
	}
	return strings.Join(alternatives, "|"), captures, nil
}

// tokenOniguruma returns the Oniguruma regex matching a single token.
func tokenOniguruma(token textMateToken) (string, error) {
	if token.Keyword != "" {
		return keywordRegex(token.Keyword), nil
	}
	regex, err := toOniguruma(token.Regexp)
	if err != nil {
		return "", fmt.Errorf("token %s: %w", token.Name, err)
	}
	return regex, nil
}

func captureCount(token textMateToken) int {
	if token.Regexp == "" {
		return 0
	}
	re, err := syntax.Parse(token.Regexp, syntax.Perl)
	if err != nil {
		return 0
	}
	return re.MaxCap()
}

// onlyWhitespace reports whether re can only match whitespace.
func onlyWhitespace(re *syntax.Regexp) bool {
	switch re.Op {
	case syntax.OpLiteral:
		return !slices.ContainsFunc(re.Rune, func(r rune) bool { return !unicode.IsSpace(r) })
	case syntax.OpCharClass:
		for i := 0; i+1 < len(re.Rune); i += 2 {
			for r := re.Rune[i]; r <= re.Rune[i+1]; r++ {
				if !unicode.IsSpace(r) {
					return false
				}
			}
		}
		return true
	case syntax.OpEmptyMatch:
		return true
	case syntax.OpCapture, syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat,
		syntax.OpConcat, syntax.OpAlternate:
		for _, sub := range re.Sub {
			if !onlyWhitespace(sub) {
				return false
			}
		}
		return true
	}
	return false
}

// hasBackslashLiteral reports whether re contains a literal backslash, which
// for strings means that the body has escape sequences.
func hasBackslashLiteral(re *syntax.Regexp) bool {
	if re.Op == syntax.OpLiteral && slices.Contains(re.Rune, '\\') {
		return true
	}
	return slices.ContainsFunc(re.Sub, hasBackslashLiteral)
}

// generateLanguage generates the TextMate grammar of one language of a
// build, with scope name [LanguageScopeName]. It only includes the language's
// entry from the common grammar (see generateCommon).
//
// fileTypes lists the extensions of the selector patterns that are simple
// extension globs (see textMateFileTypes). VS Code ignores it and uses the
// languages contribution of package.json instead.
func generateLanguage(artifacts pluginapi.BuildArtifacts, language *pluginapi.Language) (string, error) {
	return marshalTextMate(tmGrammar{
		Name:      language.LanguageID,
		ScopeName: LanguageScopeName(language.LanguageID),
		FileTypes: textMateFileTypes(language.Patterns),
		Patterns:  []tmPattern{{Include: CommonScopeName(artifacts.Grammar.Name) + "#" + language.Entry}},
	})
}

// textMateFileTypes returns the extensions (without the dot) of the patterns
// of the forms *.ext, **/*.ext and *.{ext1,ext2}, in order and without
// duplicates. Other patterns are skipped.
func textMateFileTypes(patterns []string) []string {
	var result []string
	for _, pattern := range patterns {
		rest, ok := strings.CutPrefix(pattern, "**/")
		if !ok {
			rest = pattern
		}
		rest, ok = strings.CutPrefix(rest, "*.")
		if !ok {
			continue
		}
		extensions := []string{rest}
		if inner, ok := strings.CutPrefix(rest, "{"); ok {
			if inner, ok = strings.CutSuffix(inner, "}"); !ok {
				continue
			}
			extensions = strings.Split(inner, ",")
		}
		if slices.ContainsFunc(extensions, func(ext string) bool { return ext == "" || strings.ContainsAny(ext, `*?[]{}\/,!`) }) {
			continue
		}
		for _, ext := range extensions {
			if !slices.Contains(result, ext) {
				result = append(result, ext)
			}
		}
	}
	return result
}
