// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

import (
	"testing"

	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/internal/artifacts/astinit"
)

type locatedNode struct{ location Location }

func (n *locatedNode) setLocation(l Location) { n.location = l }

func TestInitWiresSetLocation(t *testing.T) {
	require.NotNil(t, astinit.SetLocation)
	f := NewSourceFile("/a.fb", "grammar G")
	n := &locatedNode{}
	astinit.SetLocation(n, Location{Source: f, Start: 8, End: 9})
	require.Equal(t, "G", n.location.Text())
}
