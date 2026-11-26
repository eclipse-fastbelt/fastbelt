// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

// Package types describes the resolved type structure (interfaces, fields,
// supertypes) derived from a Fastbelt grammar, as handed to plugins via
// [typefox.dev/fastbelt/pluginapi.BuildArtifacts].
//
// The types are descriptive values, not Go types: there is one [Type] per
// interface declared in the grammar, in declaration order, which is also the
// order of the generated Go interfaces. Where the AST mirror only holds what
// is written in the grammar, a [Type] has the hierarchy and fields resolved:
//
//   - direct ([Type.Extends]) and transitive ([Type.AllSupertypes])
//     supertypes, and the direct [Type.Subtypes];
//   - declared ([Type.OwnFields]) and inherited ([Type.AllFields]) fields,
//     each with a [FieldType] descriptor of kind [KindPrimitive], [KindNode],
//     [KindReference] or [KindArray];
//   - the parser rules returning the type ([Type.ReturnedBy]).
//
// Types, fields and field types link back to their declarations in
// [typefox.dev/fastbelt/pluginapi/ast] ([Type.Declaration],
// [Field.Declaration], [FieldType.Declaration]), but never up to
// [typefox.dev/fastbelt/pluginapi]. Supertypes and subtypes link to each
// other, so the types form a graph with cycles.
package types
