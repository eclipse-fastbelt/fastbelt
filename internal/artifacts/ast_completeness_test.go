// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/test"
)

// astMirrorTypes lists the pluginapi/ast types that mirror a grammar
// interface, keyed by their name, which must equal the interface name.
var astMirrorTypes = typesByName(
	reflect.TypeFor[ast.Grammar](),
	reflect.TypeFor[ast.Interface](),
	reflect.TypeFor[ast.Field](),
	reflect.TypeFor[ast.FieldType](),
	reflect.TypeFor[ast.ArrayType](),
	reflect.TypeFor[ast.ReferenceType](),
	reflect.TypeFor[ast.SimpleType](),
	reflect.TypeFor[ast.PrimitiveType](),
	reflect.TypeFor[ast.AbstractRule](),
	reflect.TypeFor[ast.AbstractRuleWithBody](),
	reflect.TypeFor[ast.AbstractRuleWithReturnType](),
	reflect.TypeFor[ast.ParserRule](),
	reflect.TypeFor[ast.AbstractTokenRule](),
	reflect.TypeFor[ast.TokenDecl](),
	reflect.TypeFor[ast.TokenContent](),
	reflect.TypeFor[ast.RegexpTokenContent](),
	reflect.TypeFor[ast.KeywordTokenContent](),
	reflect.TypeFor[ast.TokenCommand](),
	reflect.TypeFor[ast.TokenGroup](),
	reflect.TypeFor[ast.TokenMode](),
	reflect.TypeFor[ast.TokenModeMember](),
	reflect.TypeFor[ast.TokenDeclUsage](),
	reflect.TypeFor[ast.TokenGroupUsage](),
	reflect.TypeFor[ast.TokenUsage](),
	reflect.TypeFor[ast.KeywordUsage](),
	reflect.TypeFor[ast.KeywordSelector](),
	reflect.TypeFor[ast.Element](),
	reflect.TypeFor[ast.Keyword](),
)

// pendingStep7 lists the grammar interfaces ("Type") and fields
// ("Type.Field") that are mirrored in step 7. Their checks are skipped.
// TODO(step 7): this list must be empty.
var pendingStep7 = map[string]bool{
	"Element":             true,
	"Alternatives":        true,
	"Group":               true,
	"Assignment":          true,
	"Assignable":          true,
	"CrossRef":            true,
	"RuleCall":            true,
	"Action":              true,
	"CompositeRule":       true,
	"InfixRule":           true,
	"PrecedenceGroup":     true,
	"Keyword.Cardinality": true,
	"Grammar.Composites":  true,
	"Grammar.InfixRules":  true,
}

func typesByName(types ...reflect.Type) map[string]reflect.Type {
	byName := make(map[string]reflect.Type, len(types))
	for _, t := range types {
		byName[t.Name()] = t
	}
	return byName
}

// grammarInterfaces parses grammar.fb and returns its interfaces by name.
func grammarInterfaces(t *testing.T) map[string]grammar.Interface {
	t.Helper()
	text, err := os.ReadFile("../grammar/grammar.fb")
	require.NoError(t, err)
	doc := test.New(t, grammar.CreateServices()).Parse(string(text)).AssertNoErrors()
	g := doc.Document.Root.(grammar.Grammar)
	ifaces := map[string]grammar.Interface{}
	for _, iface := range g.Interfaces() {
		ifaces[iface.Name()] = iface
	}
	return ifaces
}

// allFields returns the own and inherited fields of iface.
func allFields(t *testing.T, iface grammar.Interface) []grammar.Field {
	t.Helper()
	fields := append([]grammar.Field{}, iface.Fields()...)
	for _, ext := range iface.Extends() {
		super := ext.Ref(t.Context())
		require.NotNil(t, super, "%s extends unresolved %s", iface.Name(), ext.Text())
		fields = append(fields, allFields(t, super)...)
	}
	return fields
}

// isSubtype reports whether sub transitively extends super.
func isSubtype(t *testing.T, sub, super grammar.Interface) bool {
	for _, ext := range sub.Extends() {
		if s := ext.Ref(t.Context()); s == super || isSubtype(t, s, super) {
			return true
		}
	}
	return false
}

// TestASTMirrorIsComplete checks that pluginapi/ast mirrors every interface
// and field of grammar.fb: extended (abstract) interfaces as Go interfaces
// that their subtypes implement, the others as structs with a field of the
// same name and a matching kind for each own and inherited grammar field.
func TestASTMirrorIsComplete(t *testing.T) {
	ifaces := grammarInterfaces(t)
	extended := map[grammar.Interface]bool{}
	for _, iface := range ifaces {
		for _, ext := range iface.Extends() {
			extended[ext.Ref(t.Context())] = true
		}
	}
	nodeType := reflect.TypeFor[ast.Node]()

	for name, iface := range ifaces {
		if pendingStep7[name] {
			continue
		}
		goType, ok := astMirrorTypes[name]
		require.True(t, ok, "grammar interface %s has no counterpart in pluginapi/ast", name)

		if extended[iface] {
			require.Equal(t, reflect.Interface, goType.Kind(), "%s is extended and must be a Go interface", name)
			require.True(t, goType.Implements(nodeType), "%s must embed ast.Node", name)
			for subName, sub := range ifaces {
				if pendingStep7[subName] || extended[sub] || !isSubtype(t, sub, iface) {
					continue
				}
				subType := astMirrorTypes[subName]
				require.NotNil(t, subType, "grammar interface %s has no counterpart in pluginapi/ast", subName)
				require.True(t, reflect.PointerTo(subType).Implements(goType), "*%s must implement %s", subName, name)
			}
			continue
		}

		require.Equal(t, reflect.Struct, goType.Kind(), "%s must be a struct", name)
		require.True(t, reflect.PointerTo(goType).Implements(nodeType), "*%s must implement ast.Node", name)
		for _, field := range allFields(t, iface) {
			key := name + "." + field.Name()
			if pendingStep7[key] {
				continue
			}
			goField, ok := goType.FieldByName(field.Name())
			require.True(t, ok, "%s has no field %s", name, field.Name())
			require.True(t, goField.IsExported(), "%s must be exported", key)
			requireFieldKind(t, key, field.Type(), goField.Type)
		}
	}
}

// requireFieldKind checks that a Go field type matches a grammar field type:
// arrays are slices, references and contained nodes pointers to structs or
// (for abstract targets) interfaces, primitives strings or bools.
func requireFieldKind(t *testing.T, key string, fieldType grammar.FieldType, goType reflect.Type) {
	t.Helper()
	switch ft := fieldType.(type) {
	case grammar.ArrayType:
		require.Equal(t, reflect.Slice, goType.Kind(), "%s must be a slice", key)
		requireFieldKind(t, key+"[]", ft.InternalType(), goType.Elem())
	case grammar.PrimitiveType:
		want := reflect.String
		if ft.Type() == "bool" {
			want = reflect.Bool
		}
		require.Equal(t, want, goType.Kind(), "%s must be a %s", key, want)
	case grammar.ReferenceType:
		requireNodeKind(t, key, ft.Type().Ref(t.Context()).Name(), goType)
	case grammar.SimpleType:
		requireNodeKind(t, key, ft.Type().Ref(t.Context()).Name(), goType)
	default:
		t.Fatalf("%s: unknown field type %T", key, fieldType)
	}
}

func requireNodeKind(t *testing.T, key, target string, goType reflect.Type) {
	t.Helper()
	if pendingStep7[target] {
		return
	}
	want, ok := astMirrorTypes[target]
	require.True(t, ok, "%s: target %s has no counterpart in pluginapi/ast", key, target)
	if want.Kind() == reflect.Interface {
		require.Equal(t, want, goType, "%s must have type %s", key, target)
	} else {
		require.Equal(t, reflect.PointerTo(want), goType, "%s must have type *%s", key, target)
	}
}
