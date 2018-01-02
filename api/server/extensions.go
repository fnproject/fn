package server

import (
	"log"

	"github.com/fnproject/fn/fnext"
)

// TODO: Move this into `github.com/fnproject/fn` package after main is moved out of root dir.
var extensions = map[string]fnext.Extension{}

// RegisterExtension registers the extension so it's available, but does not initialize it.
// This is generally what third party extensions will use in their init() method.
func RegisterExtension(ext fnext.Extension) {
	extensions[ext.Name()] = ext
}

// AddExtensionByName This essentially just makes sure the extensions are ordered properly and is
// what the CLI uses for the `fn build-server` command. Probably not used by much else.
func (s *Server) AddExtensionByName(name string) {
	e, ok := extensions[name]
	if !ok {
		log.Fatalf("Extension %v not registered.\n", name)
	}
	err := e.Setup(s)
	if err != nil {
		log.Fatalf("Failed to add extension %v: %v\n", name, err)
	}
}

// AddExtension both registers an extension and adds it. This is useful during extension development
// or if you want to build a custom server without using `fn build-server`.
func (s *Server) AddExtension(ext fnext.Extension) {
	RegisterExtension(ext)
	s.AddExtensionByName(ext.Name())
}
