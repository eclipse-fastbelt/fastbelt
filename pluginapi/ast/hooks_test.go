// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

import (
	"testing"

	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/internal/artifacts/astinit"
)

func TestInitWiresSetLocation(t *testing.T) {
	require.NotNil(t, astinit.SetLocation)
	f := NewSourceFile("/a.fb", "grammar G")
	n := &Grammar{}
	astinit.SetLocation(n, Location{Source: f, Start: 8, End: 9})
	require.Equal(t, "G", n.Location().Text())
}

func TestInitWiresSetContainer(t *testing.T) {
	require.NotNil(t, astinit.SetContainer)
	g := &Grammar{}
	rule := &ParserRule{}
	astinit.SetContainer(rule, g)
	require.Same(t, g, rule.Container())

	// A nil container is a nil Node, not a typed nil.
	astinit.SetContainer(g, nil)
	require.Nil(t, g.Container())
}
