// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type lineCol struct{ line, col int }

// requireLineColumn checks LineColumn for an indexed and an unindexed
// (zero-value) source file, which must agree.
func requireLineColumn(t *testing.T, text string, offset int, want lineCol) {
	t.Helper()
	indexed := NewSourceFile("/a.fb", text)
	line, col := indexed.LineColumn(offset)
	require.Equal(t, want, lineCol{line, col}, "indexed, offset %d", offset)
	plain := &SourceFile{Path: "/a.fb", Text: text}
	line, col = plain.LineColumn(offset)
	require.Equal(t, want, lineCol{line, col}, "unindexed, offset %d", offset)
}

func TestLineColumnStartsAtOneOne(t *testing.T) {
	requireLineColumn(t, "grammar G", 0, lineCol{1, 1})
	requireLineColumn(t, "", 0, lineCol{1, 1})
}

func TestLineColumnMultiLine(t *testing.T) {
	text := "ab\ncd\n\nef"
	requireLineColumn(t, text, 1, lineCol{1, 2})
	requireLineColumn(t, text, 2, lineCol{1, 3}) // the \n itself
	requireLineColumn(t, text, 3, lineCol{2, 1})
	requireLineColumn(t, text, 4, lineCol{2, 2})
	requireLineColumn(t, text, 6, lineCol{3, 1})
	requireLineColumn(t, text, 7, lineCol{4, 1})
	requireLineColumn(t, text, 8, lineCol{4, 2})
}

func TestLineColumnCRLFIsOneLineBreak(t *testing.T) {
	text := "ab\r\ncd"
	requireLineColumn(t, text, 2, lineCol{1, 3}) // \r
	requireLineColumn(t, text, 3, lineCol{1, 4}) // \n of the same break
	requireLineColumn(t, text, 4, lineCol{2, 1})
	requireLineColumn(t, "a\r\n\r\nb", 5, lineCol{3, 1})
}

func TestLineColumnLoneCRIsLineBreak(t *testing.T) {
	text := "ab\rcd\r"
	requireLineColumn(t, text, 3, lineCol{2, 1})
	requireLineColumn(t, text, 5, lineCol{2, 3})
	requireLineColumn(t, text, 6, lineCol{3, 1})
	// \n\r is two line breaks, not one.
	requireLineColumn(t, "a\n\rb", 3, lineCol{3, 1})
}

func TestLineColumnCountsRunes(t *testing.T) {
	text := "x\näö€😀z"
	z := strings.Index(text, "z")
	requireLineColumn(t, text, z, lineCol{2, 5})
	requireLineColumn(t, text, len(text), lineCol{2, 6})
}

func TestLineColumnAtEndOfText(t *testing.T) {
	requireLineColumn(t, "ab\ncd", 5, lineCol{2, 3})
	requireLineColumn(t, "ab\n", 3, lineCol{2, 1})
}

func TestLineColumnClampsOffset(t *testing.T) {
	text := "ab\ncd"
	requireLineColumn(t, text, -1, lineCol{1, 1})
	requireLineColumn(t, text, -100, lineCol{1, 1})
	requireLineColumn(t, text, 6, lineCol{2, 3})
	requireLineColumn(t, text, 1000, lineCol{2, 3})
}

func TestLineColumnIndexedAndUnindexedAgree(t *testing.T) {
	text := "grammar G\r\n\rentry A: \"ä\" x=ID;\n\n// 😀\r\n"
	indexed := NewSourceFile("/a.fb", text)
	plain := &SourceFile{Path: "/a.fb", Text: text}
	for offset := -2; offset <= len(text)+2; offset++ {
		l1, c1 := indexed.LineColumn(offset)
		l2, c2 := plain.LineColumn(offset)
		require.Equal(t, lineCol{l1, c1}, lineCol{l2, c2}, "offset %d", offset)
	}
}

func TestNewSourceFileKeepsPathAndText(t *testing.T) {
	f := NewSourceFile("/dir/a.fb", "grammar G")
	require.Equal(t, "/dir/a.fb", f.Path)
	require.Equal(t, "grammar G", f.Text)
}

func TestLocationText(t *testing.T) {
	f := NewSourceFile("/a.fb", "grammar G\nentry A: x=ID;")
	require.Equal(t, "grammar", Location{Source: f, Start: 0, End: 7}.Text())
	require.Equal(t, "entry A", Location{Source: f, Start: 10, End: 17}.Text())
	require.Equal(t, "", Location{Source: f, Start: 3, End: 3}.Text())
	require.Equal(t, "", Location{Start: 0, End: 7}.Text())
	require.Equal(t, "", Location{}.Text())
}
