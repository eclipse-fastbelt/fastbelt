// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package generator

import (
	"fmt"
	"maps"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"typefox.dev/fastbelt/internal/automatons"
	"typefox.dev/fastbelt/internal/grammar"
	fbRegexp "typefox.dev/fastbelt/internal/regexp"
	"typefox.dev/fastbelt/util/codegen"
)

// GenerateLexer emits the lexer_gen.go source for the grammar. entryRules holds
// one entry rule per language, entryModes the name of the token mode each
// language starts in (empty means "default").
func GenerateLexer(grammr grammar.Grammar, entryRules []grammar.ParserRule, entryModes []string, packageName string, tokenTypes GenerateTokenTypesResult) string {
	nodes := []codegen.Node{}

	imports := map[string]bool{}
	maps.Copy(imports, tokenTypes.Imports)

	for _, tokenType := range tokenTypes.TokenTypes.All {
		nodes = append(nodes, tokenType.Code)
	}

	node := NewRootNode()
	node.AppendLine("package ", packageName)
	node.AppendLine()
	node.AppendLine("import (")
	node.Indent(func(n codegen.Node) {
		importList := make([]string, 0, len(imports))
		for imp := range imports {
			importList = append(importList, imp)
		}
		sort.Strings(importList)
		for _, imp := range importList {
			n.AppendLine(fmt.Sprintf(`"%s"`, imp))
		}
		n.AppendLine("core \"typefox.dev/fastbelt\"")
		n.AppendLine("\"typefox.dev/fastbelt/lexer\"")
		n.AppendLine("\"typefox.dev/fastbelt/util/service\"")
	})
	node.AppendLine(")")
	node.AppendLine()

	for _, n := range nodes {
		node.AppendNode(n)
		node.AppendLine()
	}

	modes, starts := LexerModes(grammr, entryRules, entryModes, tokenTypes)
	generateLexerModeEnums(node, modes)
	generateMainLexerFunction(node, modes, starts, tokenTypes)
	return FormatIfPossible(node.String())
}

func generateLexerModeEnums(node codegen.Node, modes []LexerMode) {
	node.AppendLine("const (")
	node.Indent(func(n codegen.Node) {
		for _, m := range modes {
			n.AppendLine(m.VarName, " = ", strconv.Itoa(m.Index))
		}
	})
	node.AppendLine(")")
	node.AppendLine()
}

func generateMainLexerFunction(node codegen.Node, modes []LexerMode, starts []int, tokenTypes GenerateTokenTypesResult) {
	node.AppendLine("func NewLexer(sc *service.Container) lexer.Lexer {")
	node.Indent(func(n codegen.Node) {
		n.AppendLine("modes := make([]*lexer.TokenMode, ", strconv.Itoa(len(modes)), ")")
		for _, m := range modes {
			generateTokenMode(n, m, modes, tokenTypes)
		}
		if len(starts) == 1 {
			n.AppendLine("return lexer.NewDefaultLexer(sc, ", modes[starts[0]].VarName, ", modes...)")
			return
		}
		// One entry mode per language, in the same order as the parser's entry rules.
		startNames := make([]string, len(starts))
		for i, start := range starts {
			startNames[i] = modes[start].VarName
		}
		n.AppendLine("return lexer.NewMultiLanguageLexer(sc, []int{", strings.Join(startNames, ", "), "}, modes...)")
	})
	node.AppendLine("}")
}

// generateTokenMode emits the NewTokenMode call for the given mode.
func generateTokenMode(n codegen.Node, m LexerMode, modes []LexerMode, tokenTypes GenerateTokenTypesResult) {
	n.AppendLine("modes[", m.VarName, "] = lexer.NewTokenMode(\"", m.Name, "\",")
	n.Indent(func(nn codegen.Node) {
		for _, member := range m.Members {
			generateTokenTypeUsage(nn, tokenTypes.TokenTypes.ByTokenIndex[member.TokenIndex], member, modes)
		}
	})
	n.AppendLine(")")
}

// generateTokenTypeUsage emits the UseTokenType call for a member of a token mode.
func generateTokenTypeUsage(nn codegen.Node, tokenType *TokenType, member LexerModeMember, modes []LexerMode) {
	nn.Append("lexer.UseTokenType(", tokenType.VarName, ")")
	switch member.Command {
	case "pop":
		nn.Append(".WithPopMode()")
	case "push":
		nn.Append(".WithPushMode(", modes[member.Target].VarName, ")")
	case "mode":
		nn.Append(".WithSetMode(", modes[member.Target].VarName, ")")
	}
	switch member.Modifier {
	case "comment":
		nn.Append(".WithModifier(core.CommentModifier)")
	case "hidden":
		nn.Append(".WithModifier(core.SkippedModifier)")
	}
	nn.AppendLine(",")
}

func generateKeywordTokenType(keyword grammar.Keyword, id int) GenerateLexerResult {
	code := codegen.NewNode()
	keywordValue := grammar.KeywordValue(keyword)
	code.AppendLine("const ", GeneratedTokenIdxName(keyword), " = ", strconv.Itoa(id))
	code.AppendLine()
	code.AppendLine("var ", GeneratedTokenName(keyword), " = core.NewTokenType(")
	code.Indent(func(n codegen.Node) {
		n.AppendLine(GeneratedTokenIdxName(keyword), ",")
		n.AppendLine("\"", keywordValue, "\",")
		n.AppendLine("\"", keywordValue, "\",")
		n.AppendLine("core.TokenKindKeyword,")
		n.AppendLine("func (text string, offset int) int {")
		n.Indent(func(nn codegen.Node) {
			nn.AppendLine("if strings.HasPrefix(text[offset:], \"", keywordValue, "\") {")
			nn.Indent(func(nnn codegen.Node) {
				nnn.AppendLine("return ", strconv.Itoa(len(keywordValue)))
			})
			nn.AppendLine("}")
			nn.AppendLine("return 0")
		})
		n.AppendLine("},")
		n.Append("[]rune{")
		firstRune, _ := utf8.DecodeRune([]byte(keywordValue))
		n.Append(automatons.FormatRune(firstRune))
		n.AppendLine("},")
	})
	code.Append(")")
	return GenerateLexerResult{
		Imports: map[string]bool{
			"strings": true,
		},
		Code: code,
	}
}

type GenerateLexerResult struct {
	Imports map[string]bool
	Code    codegen.Node
}

func generateTokenGroupType(tokenGroup grammar.TokenGroup, tokenGroupMembers map[string][]string, id int) GenerateLexerResult {
	code := codegen.NewNode()
	code.AppendLine("const ", GeneratedTokenIdxName(tokenGroup), " = ", strconv.Itoa(id))
	code.AppendLine()
	code.AppendLine("var ", GeneratedTokenName(tokenGroup), " = core.NewTokenGroup(")
	code.Indent(func(n codegen.Node) {
		n.AppendLine(GeneratedTokenIdxName(tokenGroup), ",")
		n.AppendLine("\"", tokenGroup.Name(), "\",")
		n.AppendLine("\"", tokenGroup.Name(), "\",")
		n.AppendLine("[]*core.TokenType{")
		for _, member := range tokenGroupMembers[tokenGroup.Name()] {
			n.AppendLine(member, ",")
		}
		n.AppendLine("},")
	})
	code.Append(")")
	return GenerateLexerResult{
		Imports: map[string]bool{},
		Code:    code,
	}
}

func generateRegexpTokenElement(token grammar.TokenDecl, regexpTokenElement grammar.RegexpTokenContent, id int) GenerateLexerResult {
	var result fbRegexp.GenerateRegExpResult
	imports := map[string]bool{}
	code := codegen.NewNode()
	regexPattern := grammar.RegexpValue(regexpTokenElement.Regexp())
	regex, err := fbRegexp.Compile(regexPattern)
	if err != nil {
		panic(err)
	}
	code.AppendLine("const ", GeneratedTokenIdxName(token), " = ", strconv.Itoa(id))
	code.AppendLine("var ", GeneratedTokenName(token), " = core.NewTokenType(")
	code.Indent(func(n codegen.Node) {
		n.AppendLine(GeneratedTokenIdxName(token), ",")
		n.AppendLine("\"", token.Name(), "\",")
		n.AppendLine("\"", token.Name(), "\",")
		n.AppendLine("core.TokenKindToken,")
		impl := regex.(*fbRegexp.RegexpImpl)
		result = impl.GenerateRegExp("", GeneratedTokenName(token))
		for imp := range result.Imports {
			imports[imp] = true
		}
		n.AppendNode(result.Code)
		n.AppendLine(",")
		n.Append("[]rune{")
		startCharsSet := impl.GetStartChars()
		n.AppendNode(runeSetToNode(startCharsSet))
		n.AppendLine("},")
	})
	code.AppendLine(")")
	code.AppendNode(result.Vars)
	return GenerateLexerResult{
		Imports: imports,
		Code:    code,
	}
}

type RuneSlice []rune

func (x RuneSlice) Len() int           { return len(x) }
func (x RuneSlice) Less(i, j int) bool { return x[i] < x[j] }
func (x RuneSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// Sort is a convenience method: x.Sort() calls Sort(x).
func (x RuneSlice) Sort() { sort.Sort(x) }

func runeSetToNode(set *automatons.RuneSet) codegen.Node {
	root := codegen.NewNode()
	for _, rng := range set.Ranges {
		if !rng.Includes {
			continue
		}
		if rng.Start == rng.End {
			root.Append(automatons.FormatRune(rng.Start), ", ")
		} else {
			for r := rng.Start; r <= rng.End; r++ {
				root.Append(automatons.FormatRune(r))
				root.Append(", ")
			}
		}
	}
	return root
}
