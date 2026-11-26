// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package pluginapi_test

import (
	"fmt"
	"os"
	"path/filepath"

	"typefox.dev/fastbelt/cmd"
	"typefox.dev/fastbelt/pluginapi"
	"typefox.dev/fastbelt/pluginapi/tokens"
)

// KeywordsPlugin prints the keywords each language's lexer starts with.
type KeywordsPlugin struct{}

func (KeywordsPlugin) Name() string { return "keywords" }

func (KeywordsPlugin) Build(artifacts pluginapi.BuildArtifacts) error {
	for _, lang := range artifacts.Languages {
		fmt.Printf("%s:", lang.LanguageID)
		for _, member := range lang.EntryMode.Members {
			if member.TokenType.Kind == tokens.KindKeyword {
				fmt.Printf(" %s", member.TokenType.Keyword)
			}
		}
		fmt.Println()
	}
	return nil
}

// This example builds a grammar with two languages and registers a minimal
// plugin that lists the keywords of each language.
func Example() {
	dir, err := os.MkdirTemp("", "pluginapi-example")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	grammar := `
		grammar Multi
		interface Greeting { Name string }
		interface Farewell { Name string }
		entry Greeting: "hello" Name=ID "!"
		entry Farewell: "goodbye" Name=ID
		token ID: /[_a-zA-Z][\w_]*/
		hidden token WS: /\s+/
	`
	if err := os.WriteFile(filepath.Join(dir, "multi.fb"), []byte(grammar), 0o644); err != nil {
		panic(err)
	}

	ctx := &cmd.BuildContext{
		Input:   dir,
		Package: "multi",
		Languages: []cmd.Language{
			{Entry: "Greeting", LanguageID: "greeting", Patterns: []string{"**/*.hello"}},
			{Entry: "Farewell", LanguageID: "farewell", Patterns: []string{"**/*.bye"}},
		},
		Plugins: []pluginapi.Plugin{KeywordsPlugin{}},
	}
	if err := ctx.Build(); err != nil {
		panic(err)
	}
	// Output:
	// greeting: hello !
	// farewell: goodbye
}
