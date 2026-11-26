// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package ast

import "typefox.dev/fastbelt/internal/artifacts/astinit"

// locationSetter is implemented by nodes with a location.
type locationSetter interface {
	setLocation(Location)
}

func init() {
	astinit.SetLocation = func(node, location any) {
		node.(locationSetter).setLocation(location.(Location))
	}
}
