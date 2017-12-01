package server

import (
	"fmt"
	"log"

	"github.com/fnproject/fn/fnext"
)

// TODO: Move this into `github.com/fnproject/fn` package after main is moved out of root dir.
var extensions = map[string]fnext.Extension{}

// RegisterExtension registers the extension so it's available, but does not initialize it or anything
func RegisterExtension(ext fnext.Extension) {
	extensions[ext.Name()] = ext
}

// AddExtensionByName This essentially just makes sure the extensions are ordered properly.
// It could do some initialization if required too.
func (s *Server) AddExtensionByName(name string) {
	fmt.Printf("extensions: %+v\n", extensions)
	e, ok := extensions[name]
	if !ok {
		log.Fatalf("Extension %v not registered.\n", name)
	}
	err := e.Setup(s)
	if err != nil {
		log.Fatalf("Failed to add extension %v: %v\n", name, err)
	}
}
