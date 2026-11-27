// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Command gen generates the statemachine language and the TextMate grammars of
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
				Entry:      "Statemachine",
				LanguageID: "statemachine",
				Patterns:   []string{"**/*.statemachine"},
			},
		},
		Plugins: []pluginapi.Plugin{textmate.New(textmate.Options{
			// Paths are relative to the output directory, which is this example.
			CommonFile: "../../internal/vscode-extensions/statemachine/syntaxes/statemachinemodel-common.tmLanguage.json",
			LanguageFiles: map[string]string{
				"statemachine": "../../internal/vscode-extensions/statemachine/syntaxes/statemachine.tmLanguage.json",
			},
		})},
		ATN:     true,
		Verbose: true,
	}
	if err := ctx.Build(); err != nil {
		log.Fatalf("statemachine generation failed: %v", err)
	}
}
