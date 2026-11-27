// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package textmate

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	require.Equal(t, "textmate", New(Options{}).Name())
}

func TestNewCopiesOptions(t *testing.T) {
	languageFiles := map[string]string{"arith": "syntaxes/arith.tmLanguage.json"}
	scopes := map[string]string{"SL_COMMENT": "comment.line"}
	p := New(Options{CommonFile: "syntaxes/common.json", LanguageFiles: languageFiles, Scopes: scopes})

	languageFiles["arith"] = "changed"
	languageFiles["other"] = "added"
	scopes["SL_COMMENT"] = "changed"
	scopes["ID"] = "added"

	require.Equal(t, "syntaxes/common.json", p.commonFile)
	require.Equal(t, map[string]string{"arith": "syntaxes/arith.tmLanguage.json"}, p.languageFiles)
	require.Equal(t, map[string]string{"SL_COMMENT": "comment.line"}, p.scopes)
}

func TestLanguageScopeName(t *testing.T) {
	require.Equal(t, "source.arith", LanguageScopeName("arith"))
	require.Equal(t, "source.My-Lang", LanguageScopeName("My-Lang"))
}

func TestCommonScopeName(t *testing.T) {
	require.Equal(t, "source.arithmetics-common", CommonScopeName("Arithmetics"))
}
