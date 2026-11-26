// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

import "typefox.dev/fastbelt/pluginapi/tokens"

// Element is an element of a rule body: *[Alternatives], *[Group],
// *[Keyword], *[Assignment], *[CrossRef], *[RuleCall] or *[Action]. Each
// has a Cardinality field.
type Element interface {
	Node
	isElement()
}

// Assignable is an element that can be the value of an [Assignment] or an
// operator of a [PrecedenceGroup]: *[Keyword], *[RuleCall], *[CrossRef] or
// *[Alternatives] (of those three).
type Assignable interface {
	Element
	isAssignable()
}

// Cardinality is the cardinality of an [Element].
type Cardinality string

const (
	// CardinalityOne is the default: the element occurs exactly once.
	CardinalityOne        Cardinality = ""
	CardinalityOptional   Cardinality = "?"
	CardinalityZeroOrMore Cardinality = "*"
	CardinalityOneOrMore  Cardinality = "+"
)

// AssignmentOperator is the operator of an [Assignment].
type AssignmentOperator string

const (
	// AssignmentSet assigns a single value: Property=Value.
	AssignmentSet AssignmentOperator = "="
	// AssignmentAppend appends to a list: Property+=Value.
	AssignmentAppend AssignmentOperator = "+="
	// AssignmentFlag sets a bool if the value was matched: Property?=Value.
	AssignmentFlag AssignmentOperator = "?="
)

// ActionOperator is the operator of an [Action].
type ActionOperator string

const (
	// ActionNone is the operator of an action without a property: {Type}.
	ActionNone ActionOperator = ""
	// ActionSet assigns the current node: {Type.Property=current}.
	ActionSet ActionOperator = "="
	// ActionAppend appends the current node: {Type.Property+=current}.
	ActionAppend ActionOperator = "+="
)

// Associativity is the associativity of a [PrecedenceGroup].
type Associativity string

const (
	// AssociativityDefault is an undeclared associativity, which the
	// generated parser treats as left-associative.
	AssociativityDefault Associativity = ""
	AssociativityLeft    Associativity = "left"
	AssociativityRight   Associativity = "right"
)

// Alternatives is a choice between elements: A | B | C.
type Alternatives struct {
	node
	Cardinality Cardinality
	// Alts are the alternatives, in declared order. Inside an assignment
	// value they are *[Keyword], *[RuleCall] or *[CrossRef].
	Alts []Element
}

// Group is a sequence of elements: A B C.
type Group struct {
	node
	Cardinality Cardinality
	Elements    []Element
	// Synthetic reports whether the group was created by infix expansion as
	// part of the body of an [InfixRule]. Its location is the one of the
	// infix rule.
	Synthetic bool
}

// Assignment assigns a matched value to a field of the current node:
// Property=Value, Property+=Value or Property?=Value.
type Assignment struct {
	node
	Cardinality Cardinality
	// Property is the assigned field of the rule's node type.
	Property *Field
	Operator AssignmentOperator
	Value    Assignable
}

// CrossRef is a cross-reference: [Type:Rule].
type CrossRef struct {
	node
	Cardinality Cardinality
	// Type is the interface of the referenced nodes.
	Type *Interface
	// Rule is the call of the token or composite rule that reads the
	// reference text. For a token rule, the cross-reference consumes the
	// token of Rule ([RuleCall.TokenType]); for a composite rule, it
	// consumes the tokens of the composite rule's body and Rule.TokenType is
	// nil. The grammar language allows to omit the rule ([Type]), but a
	// build rejects such a grammar, so Rule is never nil.
	Rule *RuleCall
}

// RuleCall is a call of a rule by name.
type RuleCall struct {
	node
	Cardinality Cardinality
	// Rule is the called rule: a parser, composite or infix rule, a token
	// declaration or a token group.
	Rule AbstractRule
	// TokenType is the token type the call consumes if Rule is a
	// *[TokenDecl] or *[TokenGroup] (their TokenType), and nil if Rule is a
	// parser, composite or infix rule.
	TokenType *tokens.TokenType
	// Synthetic reports whether the call was created by infix expansion as
	// part of the body of an [InfixRule]: an operand call, located at the
	// operand call of the infix rule, or the operator call to the synthetic
	// operator group, located at the infix rule.
	Synthetic bool
}

// Action creates a node of a type: {Type} or {Type.Property=current}.
type Action struct {
	node
	Cardinality Cardinality
	Type        *Interface
	// Operator is [ActionNone] if there is no Property.
	Operator ActionOperator
	// Property is the field of the new node the current node is assigned
	// to, or nil if there is none.
	Property *Field
}

func (*Alternatives) isElement() {}
func (*Group) isElement()        {}
func (*Keyword) isElement()      {}
func (*Assignment) isElement()   {}
func (*CrossRef) isElement()     {}
func (*RuleCall) isElement()     {}
func (*Action) isElement()       {}

func (*Alternatives) isAssignable() {}
func (*Keyword) isAssignable()      {}
func (*CrossRef) isAssignable()     {}
func (*RuleCall) isAssignable()     {}

// CompositeRule is a composite rule: composite Name: Body. Its body consists
// of [Alternatives], [Group], [Keyword] and [RuleCall] elements only.
type CompositeRule struct {
	node
	Name string
	// Body is the rule body.
	Body Element
}

func (*CompositeRule) isAbstractRule()         {}
func (*CompositeRule) isAbstractRuleWithBody() {}

// InfixRule is an infix rule: infix Name on Call returns ReturnType: Groups.
type InfixRule struct {
	node
	Name string
	// ReturnType is the declared return type, or nil if there is none.
	ReturnType *Interface
	// Call is the call of the operand rule.
	Call *RuleCall
	// Groups are the precedence groups, from the highest precedence to the
	// lowest.
	Groups []*PrecedenceGroup
	// Body is the synthetic body created by infix expansion, in the form
	// Operand (Operator Operand)*, where Operator calls the synthetic
	// operator group of the rule (see [TokenGroup.Synthetic]). All its
	// nodes are Synthetic.
	Body Element
}

func (*InfixRule) isAbstractRule()               {}
func (*InfixRule) isAbstractRuleWithBody()       {}
func (*InfixRule) isAbstractRuleWithReturnType() {}

// PrecedenceGroup is a precedence level of an [InfixRule]: Associativity
// Operators, with the operators separated by |.
type PrecedenceGroup struct {
	node
	Associativity Associativity
	// Operators are *[Keyword] or *[RuleCall] (to a token declaration or
	// token group).
	Operators []Assignable
}
