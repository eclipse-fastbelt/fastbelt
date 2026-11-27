// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

import "typefox.dev/fastbelt/internal/artifacts/astinit"

// locationSetter is implemented by nodes with a location.
type locationSetter interface {
	setLocation(Location)
}

// containerSetter is implemented by nodes with a container.
type containerSetter interface {
	setContainer(Node)
}

func init() {
	astinit.SetLocation = func(node, location any) {
		node.(locationSetter).setLocation(location.(Location))
	}
	astinit.SetContainer = func(node, container any) {
		// A nil container stays a nil Node instead of a nil any.
		c, _ := container.(Node)
		node.(containerSetter).setContainer(c)
	}
}
