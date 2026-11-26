// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

// Grammar is the root of the AST: the merged grammar of all .fb files of the
// build, after infix expansion.
//
// Its Location is zero (Source == nil) if the grammar was merged from
// several files, and covers the whole file for a single-file grammar.
type Grammar struct {
	node
	// Name is the grammar name. All files of a build declare the same name.
	Name string
	// Rules are the parser rules.
	Rules []*ParserRule
	// Composites are the composite rules.
	Composites []*CompositeRule
	// InfixRules are the infix rules.
	InfixRules []*InfixRule
	// Terminals are the top-level token declarations. Token declarations
	// inside token modes are members of the modes, see [TokenDeclUsage].
	Terminals []*TokenDecl
	// TokenGroups are the top-level token groups, including the synthetic
	// operator groups of infix rules. Token groups inside token modes are
	// members of the modes, see [TokenGroupUsage].
	TokenGroups []*TokenGroup
	// TokenModes are the declared token modes. See
	// [typefox.dev/fastbelt/pluginapi.BuildArtifacts.TokenModes] for the
	// modes the generated lexer actually runs.
	TokenModes []*TokenMode
	// Interfaces are the declared interfaces.
	Interfaces []*Interface
}

// Interface is an interface declaration: interface Name extends ... { ... }.
type Interface struct {
	node
	Name string
	// Extends are the direct super interfaces, in declared order.
	Extends []*Interface
	// Fields are the own fields, in declared order.
	Fields []*Field
}

// Field is a field of an [Interface].
type Field struct {
	node
	Name string
	Type FieldType
}

// FieldType is the type of a [Field]: one of *[ArrayType], *[ReferenceType],
// *[SimpleType] or *[PrimitiveType].
type FieldType interface {
	Node
	isFieldType()
}

// ArrayType is a list type: []InternalType.
type ArrayType struct {
	node
	InternalType FieldType
}

// ReferenceType is a cross-reference type: *Type.
type ReferenceType struct {
	node
	Type *Interface
}

// SimpleType is a contained node type: Type.
type SimpleType struct {
	node
	Type *Interface
}

// Primitive is the type name of a [PrimitiveType].
type Primitive string

const (
	PrimitiveString    Primitive = "string"
	PrimitiveBool      Primitive = "bool"
	PrimitiveComposite Primitive = "composite"
)

// PrimitiveType is a primitive type such as string.
type PrimitiveType struct {
	node
	Type Primitive
}

func (*ArrayType) isFieldType()     {}
func (*ReferenceType) isFieldType() {}
func (*SimpleType) isFieldType()    {}
func (*PrimitiveType) isFieldType() {}

// AbstractRule is any named rule: *[ParserRule], *[CompositeRule],
// *[InfixRule], *[TokenDecl] or *[TokenGroup].
type AbstractRule interface {
	Node
	isAbstractRule()
}

// AbstractRuleWithBody is a rule with a body: *[ParserRule],
// *[CompositeRule] or *[InfixRule].
type AbstractRuleWithBody interface {
	AbstractRule
	isAbstractRuleWithBody()
}

// AbstractRuleWithReturnType is a rule with an optional return type:
// *[ParserRule] or *[InfixRule].
type AbstractRuleWithReturnType interface {
	AbstractRuleWithBody
	isAbstractRuleWithReturnType()
}

// ParserRule is a parser rule: entry Name returns ReturnType: Body.
type ParserRule struct {
	node
	Name string
	// Entry reports whether the rule is marked with the entry keyword.
	Entry bool
	// ReturnType is the declared return type, or nil if there is none.
	ReturnType *Interface
	// Body is the rule body.
	Body Element
}

func (*ParserRule) isAbstractRule()               {}
func (*ParserRule) isAbstractRuleWithBody()       {}
func (*ParserRule) isAbstractRuleWithReturnType() {}
