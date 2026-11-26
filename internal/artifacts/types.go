// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package artifacts

import (
	"context"
	"errors"
	"fmt"

	"typefox.dev/fastbelt/internal/generator"
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/pluginapi/ast"
	"typefox.dev/fastbelt/pluginapi/types"
)

// Types derives the types of the grammar interfaces, in declaration order.
// g and root are the grammar and its mirror, see [Grammar].
//
// The type hierarchy and fields follow the generator ([generator.AllExtends],
// [generator.AllFields]) and the rule return types [grammar.FindReturnType],
// so they match the generated Go code.
func Types(g grammar.Grammar, root *ast.Grammar) ([]*types.Type, error) {
	ifaces := g.Interfaces()
	rules := g.Rules()
	// root is converted from g in order, see [grammarConverter.grammar].
	if len(ifaces) != len(root.Interfaces) || len(rules) != len(root.Rules) {
		return nil, errors.New("grammar mirror does not match the grammar")
	}

	result := make([]*types.Type, len(ifaces))
	byIface := make(map[grammar.Interface]*types.Type, len(ifaces))
	byName := make(map[string]*types.Type, len(ifaces))
	byDecl := make(map[*ast.Interface]*types.Type, len(ifaces))
	fields := map[grammar.Field]*types.Field{}
	for i, iface := range ifaces {
		decl := root.Interfaces[i]
		if decl.Name != iface.Name() || len(decl.Fields) != len(iface.Fields()) {
			return nil, fmt.Errorf("interface %s does not match its mirror", iface.Name())
		}
		typ := &types.Type{Name: decl.Name, Declaration: decl}
		result[i] = typ
		byIface[iface] = typ
		byName[typ.Name] = typ
		byDecl[decl] = typ
		typ.OwnFields = make([]*types.Field, len(decl.Fields))
		for j, f := range decl.Fields {
			typ.OwnFields[j] = &types.Field{Name: f.Name, Owner: typ, Declaration: f}
			fields[iface.Fields()[j]] = typ.OwnFields[j]
		}
	}

	var err error
	fail := func(e error) {
		if err == nil {
			err = e
		}
	}
	for i, iface := range ifaces {
		typ := result[i]
		for _, ext := range typ.Declaration.Extends {
			super := byDecl[ext]
			if super == nil {
				fail(fmt.Errorf("interface %s extends an interface that is not part of the grammar", typ.Name))
				continue
			}
			typ.Extends = append(typ.Extends, super)
			super.Subtypes = append(super.Subtypes, typ)
		}
		for _, name := range generator.AllExtends(g, iface) {
			super := byName[name]
			if super == nil {
				fail(fmt.Errorf("interface %s extends unknown interface %s", typ.Name, name))
				continue
			}
			typ.AllSupertypes = append(typ.AllSupertypes, super)
		}
		for _, f := range generator.AllFields(iface) {
			field := fields[f]
			if field == nil {
				fail(fmt.Errorf("interface %s inherits field %s from an interface that is not part of the grammar", typ.Name, f.Name()))
				continue
			}
			typ.AllFields = append(typ.AllFields, field)
		}
		for _, field := range typ.OwnFields {
			field.Type = fieldType(field.Declaration.Type, byDecl, func(e error) {
				fail(fmt.Errorf("field %s.%s: %w", typ.Name, field.Name, e))
			})
		}
	}

	ctx := context.Background()
	for i, r := range rules {
		returnType := grammar.FindReturnType(r, ctx)
		if returnType == nil {
			continue
		}
		typ := byIface[returnType]
		if typ == nil {
			fail(fmt.Errorf("rule %s returns %s, which is not part of the grammar", r.Name(), returnType.Name()))
			continue
		}
		typ.ReturnedBy = append(typ.ReturnedBy, root.Rules[i])
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

// fieldType describes the field type ft; targets maps the interface mirrors
// to their types.
func fieldType(ft ast.FieldType, targets map[*ast.Interface]*types.Type, fail func(error)) *types.FieldType {
	target := func(decl *ast.Interface) *types.Type {
		typ := targets[decl]
		if typ == nil {
			fail(errors.New("type is not an interface of the grammar"))
		}
		return typ
	}
	switch ft := ft.(type) {
	case *ast.PrimitiveType:
		return &types.FieldType{Kind: types.KindPrimitive, Primitive: ft.Type, Declaration: ft}
	case *ast.SimpleType:
		return &types.FieldType{Kind: types.KindNode, Target: target(ft.Type), Declaration: ft}
	case *ast.ReferenceType:
		return &types.FieldType{Kind: types.KindReference, Target: target(ft.Type), Declaration: ft}
	case *ast.ArrayType:
		return &types.FieldType{Kind: types.KindArray, Element: fieldType(ft.InternalType, targets, fail), Declaration: ft}
	default:
		fail(fmt.Errorf("unknown field type %T", ft))
		return nil
	}
}
