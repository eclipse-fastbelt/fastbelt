// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package types

import "typefox.dev/fastbelt/pluginapi/ast"

// Type is an interface declared in the grammar, with its type hierarchy and
// fields resolved.
type Type struct {
	// Name is the interface name.
	Name string
	// Declaration is the interface declaration in the grammar.
	Declaration *ast.Interface
	// Extends are the direct supertypes, in declared order.
	Extends []*Type
	// AllSupertypes are the transitive supertypes without duplicates:
	// depth-first in declared order, each supertype followed by its own
	// supertypes. The generated Impl struct embeds the Data structs of the
	// supertypes in this order.
	AllSupertypes []*Type
	// Subtypes are the direct subtypes, in grammar declaration order. It is
	// the inverse of Extends.
	Subtypes []*Type
	// OwnFields are the fields declared by this type, in declared order.
	OwnFields []*Field
	// AllFields are the fields including the inherited ones: those of the
	// supertypes first, depth-first in declared order, then OwnFields. A
	// supertype reached on several paths contributes its fields once. These
	// are the fields of the generated Go interface.
	AllFields []*Field
	// ReturnedBy are the parser rules that return this type, explicitly
	// (Rule returns Name: ...) or implicitly (a rule without returns named
	// like the type), in grammar order. Infix rules are not included; they
	// return the type of their operand rule.
	ReturnedBy []*ast.ParserRule
}

// Field is a field of a [Type].
type Field struct {
	// Name is the field name.
	Name string
	// Owner is the type that declares the field. For an inherited field in
	// [Type.AllFields], it is the supertype.
	Owner *Type
	// Type is the field type.
	Type *FieldType
	// Declaration is the field declaration in the grammar.
	Declaration *ast.Field
}

// FieldTypeKind tells what kind of value a [FieldType] describes.
type FieldTypeKind int

const (
	// KindPrimitive is a primitive value such as string, see
	// [FieldType.Primitive].
	KindPrimitive FieldTypeKind = iota
	// KindNode is a contained node of the type [FieldType.Target].
	KindNode
	// KindReference is a cross-reference to a node of the type
	// [FieldType.Target].
	KindReference
	// KindArray is a list of [FieldType.Element] values.
	KindArray
)

// String returns the name of the kind, e.g. "Reference".
func (k FieldTypeKind) String() string {
	switch k {
	case KindPrimitive:
		return "Primitive"
	case KindNode:
		return "Node"
	case KindReference:
		return "Reference"
	case KindArray:
		return "Array"
	default:
		return "FieldTypeKind(?)"
	}
}

// FieldType describes the type of a [Field].
type FieldType struct {
	// Kind tells which of the other fields is set.
	Kind FieldTypeKind
	// Primitive is the primitive type if Kind is [KindPrimitive].
	Primitive ast.Primitive
	// Target is the node type if Kind is [KindNode] or [KindReference].
	Target *Type
	// Element is the element type if Kind is [KindArray].
	Element *FieldType
	// Declaration is the field type in the grammar.
	Declaration ast.FieldType
}
