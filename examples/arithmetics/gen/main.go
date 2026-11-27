// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Command gen generates the arithmetics language and the TextMate grammars of
// its VS Code extension using the cmd.BuildContext API.
package main

import (
	"log"

	"typefox.dev/fastbelt/cmd"
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/plugins/textmate"
)

func main() {
	ctx := &cmd.BuildContext{
		Languages: []cmd.Language{
			{
				Entry:      "Module",
				LanguageID: "arithmetics",
				Patterns:   []string{"**/*.calc"},
			},
		},
		Plugins: []pluginapi.Plugin{textmate.New(textmate.Options{
			// Paths are relative to the output directory, which is this example.
			CommonFile: "../../internal/vscode-extensions/arithmetics/syntaxes/arithmetics-common.tmLanguage.json",
			LanguageFiles: map[string]string{
				"arithmetics": "../../internal/vscode-extensions/arithmetics/syntaxes/arithmetics.tmLanguage.json",
			},
			Scopes: map[string]string{
				// SL_COMMENT is hidden, so it only gets a scope through an override.
				"SL_COMMENT": "comment.line",
			},
		})},
		Verbose: true,
	}
	if err := ctx.Build(); err != nil {
		log.Fatalf("arithmetics generation failed: %v", err)
	}
}
