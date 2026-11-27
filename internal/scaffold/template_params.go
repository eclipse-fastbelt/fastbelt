// Copyright 2025 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package scaffold

import (
	"fmt"
	"go/token"
	"path"
	"strings"
	"unicode"

	"typefox.dev/fastbelt/plugins/textmate"
)

// templateParams holds derived filenames, identifiers, and metadata for a scaffolded language module or package.
type templateParams struct {
	CreateVSCodeExtension bool
	// ModulePath is the Go module path for a new module (go mod init), or the import path of the
	// language package when scaffolding into an existing module.
	ModulePath string
	// ModuleDirBase is the last path segment of ModulePath (output directory name).
	ModuleDirBase string
	// LanguageLabel is the display name supplied by the user.
	LanguageLabel string
	// LanguageID is the VS Code language identifier (lowercase slug).
	LanguageID string
	// GoPackage is the root Go package name (language implementation package).
	GoPackage string
	// ExportedBase is the PascalCase prefix for user-visible Go types (for example MyLangSrv).
	ExportedBase string
	// GrammarName is the grammar declaration name in the .fb file (ExportedBase + "Model").
	GrammarName string
	// GrammarFile is the relative path to the .fb grammar file at the module root.
	GrammarFile string
	// FileDotExt is the document file extension configured for the workspace (including a leading dot).
	FileDotExt string
	// VSCodeFileExt is the file extension without a leading dot for contributes.languages.
	VSCodeFileExt string
	// ScopeName is the TextMate scope name of the generated language grammar (source.<LanguageID>, textmate.LanguageScopeName).
	ScopeName string
	// LSPSlug is the subdirectory name cmd/<LSPSlug> for the LSP main package.
	LSPSlug string
	// SyntaxFile is the basename of the generated language grammar under vscode-extension/syntaxes.
	// gen/main.go passes it to the textmate plugin.
	SyntaxFile string
	// CommonScopeName is the TextMate scope name of the generated common grammar
	// (source.<lower(GrammarName)>-common, textmate.CommonScopeName).
	CommonScopeName string
	// CommonSyntaxFile is the basename of the generated common grammar under vscode-extension/syntaxes.
	// gen/main.go passes it to the textmate plugin.
	CommonSyntaxFile string
	// NPMExtension is the vscode-extension npm package name (alnum + "-vscode").
	NPMExtension string
	// VSIXFilename is the output .vsix file name produced by vsce package.
	VSIXFilename string
	// RootNPMName is the npm package name for the root workspace package.json.
	RootNPMName string
	// FastbeltGoGet is the typefox.dev/fastbelt/cmd package path with version for go get (P@V).
	// Getting the package rather than the module also records the go.sum entries of its imports,
	// which go run ./gen needs before go mod tidy runs.
	FastbeltGoGet string
}

func newTemplateParams(s *Scaffolder) (templateParams, error) {
	modulePath := s.ImportPath
	modulePath = strings.TrimSpace(modulePath)
	if modulePath == "" {
		return templateParams{}, fmt.Errorf("module path is empty")
	}
	// Module paths use slash-separated import syntax; use path, not filepath (Windows).
	base := path.Base(modulePath)
	if base == "." || base == "/" {
		return templateParams{}, fmt.Errorf("invalid module path %q", modulePath)
	}
	languageLabel := s.Language
	languageLabel = strings.TrimSpace(languageLabel)
	languageLabel = strings.ToUpper(languageLabel[0:1]) + languageLabel[1:]
	if languageLabel == "" {
		return templateParams{}, fmt.Errorf("language name is empty")
	}

	languageID := deriveLanguageID(languageLabel)
	exported := exportedBase(languageLabel)
	if exported == "" {
		exported = "Lang"
	}
	grammarName := exported + "Model"
	goPkg := goPackageName(languageID)
	grammarFile := languageID + ".fb"
	alnumExt := strings.ReplaceAll(languageID, "-", "")
	if alnumExt == "" {
		alnumExt = "lang"
	}
	dotExt := "." + alnumExt
	lspSlug := languageID + "-lsp"
	// The TextMate grammar files are named after the language ID and the lower-cased grammar name.
	syntaxFile := languageID + ".tmLanguage.json"
	commonSyntaxFile := strings.ToLower(grammarName) + "-common.tmLanguage.json"
	npmExt := alnumExt + "-vscode"
	vsix := npmExt + ".vsix"

	fastbeltGoGet := fastbeltModulePath + "/cmd"
	if localPath := fastbeltModuleLocalPath(); localPath == "" {
		// If no local path override is set, use the resolved module version for go get.
		ver := fastbeltModuleVersion()
		fastbeltGoGet += "@" + ver
	}

	return templateParams{
		CreateVSCodeExtension: s.CreateVSCodeExtension,
		ModulePath:            modulePath,
		ModuleDirBase:         base,
		LanguageLabel:         languageLabel,
		LanguageID:            languageID,
		GoPackage:             goPkg,
		ExportedBase:          exported,
		GrammarName:           grammarName,
		GrammarFile:           grammarFile,
		FileDotExt:            dotExt,
		VSCodeFileExt:         alnumExt,
		ScopeName:             textmate.LanguageScopeName(languageID),
		LSPSlug:               lspSlug,
		SyntaxFile:            syntaxFile,
		CommonScopeName:       textmate.CommonScopeName(grammarName),
		CommonSyntaxFile:      commonSyntaxFile,
		NPMExtension:          npmExt,
		VSIXFilename:          vsix,
		RootNPMName:           alnumExt + "-workspace",
		FastbeltGoGet:         fastbeltGoGet,
	}, nil
}

func deriveLanguageID(languageLabel string) string {
	var b strings.Builder
	prevSep := true
	for _, r := range strings.ToLower(languageLabel) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prevSep = false
			continue
		}
		if r == '-' || r == '_' || unicode.IsSpace(r) {
			if !prevSep {
				b.WriteRune('-')
			}
			prevSep = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if s == "" {
		return "language"
	}
	return s
}

func exportedBase(languageLabel string) string {
	parts := strings.FieldsFunc(languageLabel, func(r rune) bool {
		return r == '-' || r == '_' || unicode.IsSpace(r)
	})
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		lower := strings.ToLower(p)
		r := []rune(lower)
		if len(r) == 0 {
			continue
		}
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

func goPackageName(languageID string) string {
	s := strings.ReplaceAll(languageID, "-", "")
	if s == "" {
		s = "language"
	}
	if !token.IsKeyword(s) && s != "true" && s != "false" && s != "iota" {
		return s
	}
	return s + "lang"
}
