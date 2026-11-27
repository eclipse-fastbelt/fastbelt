// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

import (
	"slices"
	"unicode/utf8"
)

// SourceFile is a .fb grammar file of the build.
//
// Create it with [NewSourceFile], which precomputes the line index used by
// [SourceFile.LineColumn]. A zero-value index works too, just slower.
type SourceFile struct {
	// Path is the absolute path of the .fb file.
	Path string
	// Text is the full file content.
	Text string
	// lineStarts holds the byte offset of each line start; lineStarts[0] == 0.
	lineStarts []int
}

// NewSourceFile returns a source file with a precomputed line index.
func NewSourceFile(path, text string) *SourceFile {
	return &SourceFile{Path: path, Text: text, lineStarts: computeLineStarts(text)}
}

// LineColumn returns the 1-based line and the 1-based column of a byte offset
// into Text, for example to print "path:line:col" messages.
//
// Note the units: offset is in bytes (like [Location]), the column is in
// runes. Line breaks are \n, \r\n and \r; offsets inside a line break belong
// to the line it ends. offset is clamped to [0, len(Text)].
func (f *SourceFile) LineColumn(offset int) (line, col int) {
	offset = max(0, min(offset, len(f.Text)))
	starts := f.lineStarts
	if starts == nil {
		// Not cached: the file is shared read-only between plugins.
		starts = computeLineStarts(f.Text)
	}
	i, found := slices.BinarySearch(starts, offset)
	if !found {
		i--
	}
	return i + 1, utf8.RuneCountInString(f.Text[starts[i]:offset]) + 1
}

// computeLineStarts returns the byte offset of each line start in text.
func computeLineStarts(text string) []int {
	starts := []int{0}
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '\r':
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			starts = append(starts, i+1)
		case '\n':
			starts = append(starts, i+1)
		}
	}
	return starts
}

// Location is a byte range in a source file.
//
// Start and End are byte offsets into Source.Text (Start inclusive, End
// exclusive), as produced by the parser, so that Source.Text[Start:End] is the
// located text. Use [SourceFile.LineColumn] to turn an offset into a line and
// a rune column.
type Location struct {
	// Source is the file the range points into. It is nil for nodes without a
	// single source, such as the merged Grammar root of a multi-file grammar.
	Source *SourceFile
	// Start is the inclusive start byte offset.
	Start int
	// End is the exclusive end byte offset.
	End int
}

// Text returns the located text, Source.Text[Start:End], or "" if Source is
// nil.
func (l Location) Text() string {
	if l.Source == nil {
		return ""
	}
	return l.Source.Text[l.Start:l.End]
}
