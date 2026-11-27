// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package textmate is a Fastbelt plugin that generates TextMate grammars for
// syntax highlighting from the token modes of a Fastbelt build.
//
// It writes one common grammar for all languages of the build, with scope
// name [CommonScopeName], and one grammar per configured language, with
// scope name [LanguageScopeName], that includes the language's entry rule
// from the common grammar.
//
// Create the plugin with [New] and register it in
// [typefox.dev/fastbelt/cmd.BuildContext.Plugins]:
//
//	textmate.New(textmate.Options{
//	    CommonFile:    "syntaxes/arithmetics-common.tmLanguage.json",
//	    LanguageFiles: map[string]string{"arithmetics": "syntaxes/arithmetics.tmLanguage.json"},
//	    Scopes:        map[string]string{"SL_COMMENT": "comment.line"},
//	})
//
// Relative paths are resolved against
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts.OutputDir].
package textmate
