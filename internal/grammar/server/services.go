// Copyright 2026 TypeFox GmbH
// This program and the accompanying materials are made available under the
// terms of the MIT License, which is available in the project root.

package main

import (
	"typefox.dev/fastbelt/internal/grammar"
	"typefox.dev/fastbelt/server"
	"typefox.dev/fastbelt/util/service"
)

// CreateServices creates a service container for the grammar language to be
// used in the language server. The setup function registers the transport and
// may override services for the platform the server runs on.
func CreateServices(setup func(sc *service.Container)) *service.Container {
	sc := service.NewContainer()
	grammar.SetupServices(sc)
	grammar.SetupGeneratedServerServices(sc)
	sc.Put(grammar.NewHoverProvider(sc))
	// All .fb files of a folder form one grammar, so the folder of an opened
	// file is loaded even if it is not part of a workspace folder.
	sc.Put(grammar.NewOpenDirectoryLoader(sc))
	server.SetupDefaultServices(sc)
	setup(sc)
	sc.Seal()
	return sc
}
