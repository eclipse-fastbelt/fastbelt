// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package textmate is a Fastbelt plugin that generates TextMate grammars for
// syntax highlighting in editors like VS Code from the token modes of a
// Fastbelt build.
//
// # Usage
//
// The fastbelt CLI does not run plugins; use a small generator program with
// [typefox.dev/fastbelt/cmd.BuildContext] instead, run through
// `go generate`. Scaffolded projects contain one in `gen/main.go`. Create the
// plugin with [New] and register it in
// [typefox.dev/fastbelt/cmd.BuildContext.Plugins]:
//
//	ctx := &cmd.BuildContext{
//	    Languages: []cmd.Language{{
//	        Entry:      "Model",
//	        LanguageID: "mylang",
//	        Patterns:   []string{"**/*.mylang"},
//	    }},
//	    Plugins: []pluginapi.Plugin{textmate.New(textmate.Options{
//	        CommonFile:    "vscode-extension/syntaxes/mylangmodel-common.tmLanguage.json",
//	        LanguageFiles: map[string]string{"mylang": "vscode-extension/syntaxes/mylang.tmLanguage.json"},
//	        Scopes:        map[string]string{"SL_COMMENT": "comment.line"},
//	    })},
//	}
//	err := ctx.Build()
//
// # Options
//
// [Options] has three fields:
//
//   - CommonFile is the path of the common grammar. It is required.
//   - LanguageFiles maps language IDs to the path of their grammar. A
//     language of the build without an entry gets no grammar file. An entry
//     for a language ID that is not part of the build is an error.
//   - Scopes overrides the scopes derived from token names (see Scopes
//     below).
//
// Relative paths are resolved against
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts.OutputDir]; absolute paths
// are used as is. Parent directories are created. The build fails if two
// paths are the same file, including a language file equal to the common
// file. These checks run after the Go code has been written.
//
// # Files
//
// The plugin writes two kinds of grammars:
//
//   - The common grammar with scope name `source.<grammar>-common` (see
//     [CommonScopeName]), where `<grammar>` is the lower-cased grammar name.
//     It contains the rules of all languages of the build, with one
//     repository entry per language entry rule.
//   - One grammar per configured language with scope name
//     `source.<languageID>` (see [LanguageScopeName]). It only includes the
//     language's entry rule from the common grammar. Its fileTypes are taken
//     from patterns like `**/*.mylang` and `*.{a,b}`; VS Code ignores them.
//
// The build fails if the scope name of a language equals the common scope
// name.
//
// # Scopes
//
// Only the lexical level of the grammar is used: keywords and token rules,
// with their modifiers and token modes. Parser rules only matter as entry
// rules, which select each language's entry token mode.
//
// Every keyword and token gets a TextMate scope from its name and modifier.
// All scopes end in the lower-cased grammar name, e.g. keyword.control.mylang,
// because the rules are shared by all languages of the grammar. The rules
// apply in this order:
//
//   - An override from Options.Scopes wins (see below).
//   - Hidden tokens get no scope.
//   - Comment tokens get comment.block if they have a begin and an end
//     delimiter (see below), otherwise comment.line.
//   - Keywords that contain a letter get keyword.control. Of the other
//     keywords, ";" gets punctuation.terminator, keywords of only ",.:" get
//     punctuation.separator, keywords of only "(){}[]" get
//     punctuation.section, and the rest keyword.operator. A token rule whose
//     content is a keyword, as in `token ARROW: "=>"`, counts as a keyword.
//   - Token names are split into words at underscores, digits and case
//     changes (ML_COMMENT, stringLiteral). A word STRING or STR gives
//     string.quoted.double, .single, .backtick or .other, depending on the
//     opening quote. A word INT, INTEGER, NUMBER, NUM, FLOAT, DOUBLE, DECIMAL,
//     REAL or HEX gives constant.numeric. Whole words are compared, so POINT
//     is not a number.
//   - Everything else, e.g. ID, gets no scope.
//
// Overrides map a token name to a scope without the grammar suffix, e.g.
// "ID" to "variable.other". An empty scope removes the scope. Overrides also
// apply to hidden tokens, which is how a hidden comment gets highlighted:
//
//	hidden token SL_COMMENT: /\/\/[^\r\n]*/   // override "SL_COMMENT": "comment.line"
//
// Keywords without a token rule are named like the generated Keyword_*
// constants without the prefix: "if" for `if`, "Semicolon" for ";".
//
// # Warnings
//
// The plugin reports problems that do not stop the build through
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts.Logger]:
//
//   - a scope override for a token name that is neither a token nor a
//     keyword of the grammar;
//   - a token that switches between token modes with different pop tokens
//     (see Token Modes below).
//
// By default, the build prints them as `Warning: <message>` to stderr; set
// [typefox.dev/fastbelt/cmd.BuildContext.Logger] to handle them differently.
//
// # Approximating the Lexer
//
// TextMate matches one line at a time and takes the leftmost match, breaking
// ties by rule order, while the lexer takes the longest match. The generated
// rules approximate the lexer as follows:
//
//   - Regular expressions are converted from Go syntax to the Oniguruma
//     syntax used by TextMate, with flags such as (?s) and (?m) resolved.
//   - All keywords of a token mode form one rule, longest keyword first, with
//     word boundaries only where a keyword starts or ends with a word
//     character. So "if" does not highlight the start of "iffy", and ":="
//     wins over ":".
//   - Comment, hidden and string tokens come before keywords, so "//" wins
//     over a "/" keyword. Keywords come before the other tokens, so "if" wins
//     over ID.
//   - Comment and string tokens that span lines must be split into a literal
//     begin and end, such as "/*" and "*/". The plugin does this for each
//     alternative of the regular expression; alternatives without an end
//     delimiter end at the end of the line. Tokens without a literal begin, or
//     with more than 32 alternatives, stay single-line rules. Strings whose
//     expression contains a literal backslash highlight a backslash and the
//     character after it as constant.character.escape.
//   - Hidden tokens that only match whitespace are left out.
//
// A TextMate rule has one scope per match, so text inside a token, such as
// the parts of a regular expression literal, cannot be highlighted
// separately. Languages that need that keep a hand-written TextMate grammar.
//
// # Token Modes
//
// Token modes (see [typefox.dev/fastbelt/lexer]) map to TextMate rules that
// contain other rules:
//
//   - A token that pushes mode M becomes a rule that begins at that token,
//     contains the rules of M, and ends at any pop token of M. If M has no
//     pop token, the rule runs to the end of the file.
//   - Pop tokens are only highlighted as the end of that rule, never inside
//     their own mode. Pop tokens of an entry mode are therefore not
//     highlighted at all.
//   - TextMate has no equivalent of the mode command. A token that switches
//     from mode P to mode M becomes a rule that contains the rules of M and
//     ends just before a pop token of P, where the enclosing rule ends (at
//     the end of the file if P has none). If the pop tokens of M and P
//     differ, the plugin logs a warning, because the highlighting may then
//     end at the wrong place.
//
// # VS Code Extension
//
// The grammars are generated, but the extension's `package.json` is not. It
// needs one `grammars` entry per language and one for the common grammar,
// without a language, so that the language grammars can include it:
//
//	"contributes": {
//	  "grammars": [
//	    {
//	      "language": "mylang",
//	      "scopeName": "source.mylang",
//	      "path": "./syntaxes/mylang.tmLanguage.json"
//	    },
//	    {
//	      "scopeName": "source.mylangmodel-common",
//	      "path": "./syntaxes/mylangmodel-common.tmLanguage.json"
//	    }
//	  ]
//	}
//
// Here the grammar is named `MyLangModel`. The language ID must match the
// `languages` entry of `package.json`.
package textmate
