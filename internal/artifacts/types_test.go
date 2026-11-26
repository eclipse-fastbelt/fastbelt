// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	core "typefox.dev/fastbelt"
	"typefox.dev/fastbelt/examples/arithmetics"
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/pluginapi/types"
)

const typesGrammar = `grammar Types;

interface Named { Name string }
interface Base { Id string }
interface Left extends Base { L string }
interface Right extends Base { R string }
interface Diamond extends Left, Right, Named { D bool }
interface Holder {
    Path composite
    Flag bool
    Child Named
    Target *Named
    Children []Named
    Targets []*Named
    Words []string
}

entry Holder: "holder" Path=QName Flag?="flag"? Child=Diamond ("ref" Target=[Named:ID])?
    Children+=Diamond* ("refs" Targets+=[Named:ID])* ("words" Words+=ID)*;
Diamond: "d" Name=ID Id=ID L=ID R=ID D?="x"?;
Other returns Named: "o" Name=ID;
composite QName: ID ("." ID)*;

token ID: /[a-z]+/;
hidden token WS: /\s+/;
`

// typesOf converts a single-file grammar and derives its types.
func typesOf(t *testing.T, text string) (converted, []*types.Type) {
	t.Helper()
	c := grammarOf(t, text)
	ts, err := Types(c.g, c.root)
	require.NoError(t, err)
	requireTypeInvariants(t, c.root, ts)
	return c, ts
}

// requireTypeInvariants asserts what holds for every type list: one type per
// interface in declaration order, linked to its declaration, with Subtypes
// the inverse of Extends and fields owned by their declaring type.
func requireTypeInvariants(t *testing.T, root *ast.Grammar, ts []*types.Type) {
	t.Helper()
	require.Len(t, ts, len(root.Interfaces))
	byDecl := map[*ast.Interface]*types.Type{}
	for i, typ := range ts {
		decl := root.Interfaces[i]
		require.Same(t, decl, typ.Declaration)
		require.Equal(t, decl.Name, typ.Name)
		byDecl[decl] = typ
	}
	for _, typ := range ts {
		require.Len(t, typ.Extends, len(typ.Declaration.Extends), typ.Name)
		for i, ext := range typ.Extends {
			require.Same(t, byDecl[typ.Declaration.Extends[i]], ext, typ.Name)
			require.Contains(t, ext.Subtypes, typ, "%s is a subtype of %s", typ.Name, ext.Name)
		}
		for _, sub := range typ.Subtypes {
			require.Contains(t, sub.Extends, typ, "%s extends %s", sub.Name, typ.Name)
		}
		require.Len(t, typ.OwnFields, len(typ.Declaration.Fields), typ.Name)
		for i, f := range typ.OwnFields {
			decl := typ.Declaration.Fields[i]
			require.Same(t, decl, f.Declaration)
			require.Equal(t, decl.Name, f.Name)
			require.Same(t, typ, f.Owner)
			require.Same(t, decl.Type, f.Type.Declaration)
		}
		for _, f := range typ.AllFields {
			require.True(t, f.Owner == typ || slices.Contains(typ.AllSupertypes, f.Owner),
				"field %s of %s is owned by %s", f.Name, typ.Name, f.Owner.Name)
			require.Contains(t, f.Owner.OwnFields, f)
		}
		for _, r := range typ.ReturnedBy {
			require.Contains(t, root.Rules, r)
		}
	}
}

func typeNamed(t *testing.T, ts []*types.Type, name string) *types.Type {
	return named(t, ts, name, func(typ *types.Type) string { return typ.Name })
}

// namesOf maps items to their names.
func namesOf[T any](items []T, nameOf func(T) string) []string {
	result := make([]string, len(items))
	for i, item := range items {
		result[i] = nameOf(item)
	}
	return result
}

func typeNames(ts []*types.Type) []string {
	return namesOf(ts, func(typ *types.Type) string { return typ.Name })
}

func fieldNames(fs []*types.Field) []string {
	return namesOf(fs, func(f *types.Field) string { return f.Name })
}

func ruleNames(rs []*ast.ParserRule) []string {
	return namesOf(rs, func(r *ast.ParserRule) string { return r.Name })
}

func typeField(t *testing.T, typ *types.Type, name string) *types.Field {
	return named(t, typ.AllFields, name, func(f *types.Field) string { return f.Name })
}

func TestTypesInDeclarationOrder(t *testing.T) {
	c, ts := typesOf(t, typesGrammar)
	require.Equal(t, []string{"Named", "Base", "Left", "Right", "Diamond", "Holder"}, typeNames(ts))
	require.Same(t, iface(t, c.root, "Diamond"), typeNamed(t, ts, "Diamond").Declaration)
}

func TestTypesFieldTypes(t *testing.T) {
	_, ts := typesOf(t, typesGrammar)
	holder := typeNamed(t, ts, "Holder")
	named := typeNamed(t, ts, "Named")
	require.Equal(t, []string{"Path", "Flag", "Child", "Target", "Children", "Targets", "Words"}, fieldNames(holder.OwnFields))

	path := typeField(t, holder, "Path").Type
	require.Equal(t, types.KindPrimitive, path.Kind)
	require.Equal(t, ast.PrimitiveComposite, path.Primitive)
	require.Nil(t, path.Target)
	require.Nil(t, path.Element)

	flag := typeField(t, holder, "Flag").Type
	require.Equal(t, types.KindPrimitive, flag.Kind)
	require.Equal(t, ast.PrimitiveBool, flag.Primitive)

	child := typeField(t, holder, "Child").Type
	require.Equal(t, types.KindNode, child.Kind)
	require.Same(t, named, child.Target)
	require.Empty(t, child.Primitive)
	require.Nil(t, child.Element)

	target := typeField(t, holder, "Target").Type
	require.Equal(t, types.KindReference, target.Kind)
	require.Same(t, named, target.Target)

	children := typeField(t, holder, "Children").Type
	require.Equal(t, types.KindArray, children.Kind)
	require.Nil(t, children.Target)
	require.Equal(t, types.KindNode, children.Element.Kind)
	require.Same(t, named, children.Element.Target)
	require.Same(t, children.Declaration.(*ast.ArrayType).InternalType, children.Element.Declaration)

	targets := typeField(t, holder, "Targets").Type
	require.Equal(t, types.KindArray, targets.Kind)
	require.Equal(t, types.KindReference, targets.Element.Kind)
	require.Same(t, named, targets.Element.Target)
	require.Same(t, targets.Declaration.(*ast.ArrayType).InternalType, targets.Element.Declaration)

	words := typeField(t, holder, "Words").Type
	require.Equal(t, types.KindArray, words.Kind)
	require.Equal(t, types.KindPrimitive, words.Element.Kind)
	require.Equal(t, ast.PrimitiveString, words.Element.Primitive)
}

func TestTypesSingleInheritance(t *testing.T) {
	_, ts := typesOf(t, typesGrammar)
	left := typeNamed(t, ts, "Left")
	base := typeNamed(t, ts, "Base")
	require.Equal(t, []*types.Type{base}, left.Extends)
	require.Equal(t, []*types.Type{base}, left.AllSupertypes)
	require.Equal(t, []string{"L"}, fieldNames(left.OwnFields))
	require.Equal(t, []string{"Id", "L"}, fieldNames(left.AllFields))
	require.Same(t, base.OwnFields[0], left.AllFields[0])
	require.Same(t, base, left.AllFields[0].Owner)
	require.Empty(t, base.Extends)
	require.Empty(t, base.AllSupertypes)
}

func TestTypesMultipleInheritanceDiamond(t *testing.T) {
	_, ts := typesOf(t, typesGrammar)
	diamond := typeNamed(t, ts, "Diamond")
	require.Equal(t, []string{"Left", "Right", "Named"}, typeNames(diamond.Extends))
	// Base is reachable via Left and Right but listed once.
	require.Equal(t, []string{"Left", "Base", "Right", "Named"}, typeNames(diamond.AllSupertypes))
	require.Equal(t, []string{"Id", "L", "R", "Name", "D"}, fieldNames(diamond.AllFields))
	require.Equal(t, []string{"D"}, fieldNames(diamond.OwnFields))
	owners := make([]string, len(diamond.AllFields))
	for i, f := range diamond.AllFields {
		owners[i] = f.Owner.Name
	}
	require.Equal(t, []string{"Base", "Left", "Right", "Named", "Diamond"}, owners)
	require.Same(t, typeNamed(t, ts, "Base").OwnFields[0], diamond.AllFields[0])
}

func TestTypesSubtypes(t *testing.T) {
	_, ts := typesOf(t, typesGrammar)
	require.Equal(t, []string{"Left", "Right"}, typeNames(typeNamed(t, ts, "Base").Subtypes))
	require.Equal(t, []string{"Diamond"}, typeNames(typeNamed(t, ts, "Left").Subtypes))
	require.Equal(t, []string{"Diamond"}, typeNames(typeNamed(t, ts, "Named").Subtypes))
	require.Empty(t, typeNamed(t, ts, "Diamond").Subtypes)
	require.Empty(t, typeNamed(t, ts, "Holder").Subtypes)
}

func TestTypesReturnedBy(t *testing.T) {
	c, ts := typesOf(t, typesGrammar)
	// Implicit: the rule is named like the interface.
	require.Equal(t, []*ast.ParserRule{rule(t, c.root, "Holder")}, typeNamed(t, ts, "Holder").ReturnedBy)
	require.Equal(t, []*ast.ParserRule{rule(t, c.root, "Diamond")}, typeNamed(t, ts, "Diamond").ReturnedBy)
	// Explicit: returns Named.
	require.Equal(t, []*ast.ParserRule{rule(t, c.root, "Other")}, typeNamed(t, ts, "Named").ReturnedBy)
	require.Empty(t, typeNamed(t, ts, "Base").ReturnedBy)
}

func TestTypesOfArithmetics(t *testing.T) {
	c := grammarOfFile(t, "../../examples/arithmetics/arithmetics.fb")
	ts, err := Types(c.g, c.root)
	require.NoError(t, err)
	requireTypeInvariants(t, c.root, ts)

	require.Equal(t, []string{"Expression", "PrimaryExpression"}, ruleNames(typeNamed(t, ts, "Expression").ReturnedBy))
	require.Equal(t, []string{"Statement"}, ruleNames(typeNamed(t, ts, "Statement").ReturnedBy))
	// Only the infix rule creates BinaryExpression nodes; it is not a parser
	// rule (and its return type is the operand's type, Expression).
	require.Empty(t, typeNamed(t, ts, "BinaryExpression").ReturnedBy)
	require.Equal(t, []string{"Definition", "DeclaredParameter"}, typeNames(typeNamed(t, ts, "AbstractDefinition").Subtypes))
}

// generatedArithmetics are the Go interfaces and Impl structs generated for
// examples/arithmetics.
var generatedArithmetics = map[string][2]reflect.Type{
	"Module":             {reflect.TypeFor[arithmetics.Module](), reflect.TypeFor[arithmetics.ModuleImpl]()},
	"Statement":          {reflect.TypeFor[arithmetics.Statement](), reflect.TypeFor[arithmetics.StatementImpl]()},
	"AbstractDefinition": {reflect.TypeFor[arithmetics.AbstractDefinition](), reflect.TypeFor[arithmetics.AbstractDefinitionImpl]()},
	"Definition":         {reflect.TypeFor[arithmetics.Definition](), reflect.TypeFor[arithmetics.DefinitionImpl]()},
	"DeclaredParameter":  {reflect.TypeFor[arithmetics.DeclaredParameter](), reflect.TypeFor[arithmetics.DeclaredParameterImpl]()},
	"Evaluation":         {reflect.TypeFor[arithmetics.Evaluation](), reflect.TypeFor[arithmetics.EvaluationImpl]()},
	"Expression":         {reflect.TypeFor[arithmetics.Expression](), reflect.TypeFor[arithmetics.ExpressionImpl]()},
	"BinaryExpression":   {reflect.TypeFor[arithmetics.BinaryExpression](), reflect.TypeFor[arithmetics.BinaryExpressionImpl]()},
	"FunctionCall":       {reflect.TypeFor[arithmetics.FunctionCall](), reflect.TypeFor[arithmetics.FunctionCallImpl]()},
	"NumberLiteral":      {reflect.TypeFor[arithmetics.NumberLiteral](), reflect.TypeFor[arithmetics.NumberLiteralImpl]()},
}

// TestTypesMatchGeneratedArithmetics compares the types with the Go code
// generated for examples/arithmetics: AllFields are exactly the getters of
// the Go interface, and AllSupertypes are the Data structs the Impl struct
// embeds, in order.
func TestTypesMatchGeneratedArithmetics(t *testing.T) {
	c := grammarOfFile(t, "../../examples/arithmetics/arithmetics.fb")
	ts, err := Types(c.g, c.root)
	require.NoError(t, err)
	require.Len(t, ts, len(generatedArithmetics))
	astNode := reflect.TypeFor[core.AstNode]()

	for _, typ := range ts {
		generated, ok := generatedArithmetics[typ.Name]
		require.True(t, ok, typ.Name)
		goIface, impl := generated[0], generated[1]

		// Getters are the methods that are not inherited from core.AstNode,
		// type markers, setters or token/node accessors.
		markers := map[string]bool{"Is" + typ.Name: true}
		for _, super := range typ.AllSupertypes {
			markers["Is"+super.Name] = true
		}
		accessors := map[string]bool{}
		for _, f := range typ.AllFields {
			accessors[f.Name+"Token"] = true
			accessors[f.Name+"Node"] = true
		}
		var getters []string
		for m := range goIface.Methods() {
			if _, ok := astNode.MethodByName(m.Name); ok || markers[m.Name] || accessors[m.Name] || strings.HasPrefix(m.Name, "Set") {
				continue
			}
			getters = append(getters, m.Name)
		}
		var want []string
		for _, f := range typ.AllFields {
			getter := f.Name
			if f.Type.Kind == types.KindPrimitive && f.Type.Primitive == ast.PrimitiveBool {
				getter = "Is" + getter
			}
			want = append(want, getter)
		}
		require.ElementsMatch(t, want, getters, typ.Name)

		var embedded []string
		for f := range impl.Fields() {
			if name, ok := strings.CutSuffix(f.Name, "Data"); ok && f.Anonymous {
				embedded = append(embedded, name)
			}
		}
		require.Equal(t, append(typeNames(typ.AllSupertypes), typ.Name), embedded, typ.Name)
	}
}
