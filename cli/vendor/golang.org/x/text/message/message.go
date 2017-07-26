// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package message implements formatted I/O for localized strings with functions
// analogous to the fmt's print functions.
//
// NOTE: Under construction. See https://golang.org/design/12750-localization
// and its corresponding proposal issue https://golang.org/issues/12750.
package message // import "golang.org/x/text/message"

import (
	"io"
	"os"

	"golang.org/x/text/language"
	"golang.org/x/text/message/catalog"
)

// A Printer implements language-specific formatted I/O analogous to the fmt
// package. Only one goroutine may use a Printer at the same time.
type Printer struct {
	// Wrap the fields in a hidden type to hide some of the implemented methods.
	printer printer

	// NOTE: limiting one goroutine per Printer allows for many optimizations
	// and simplifications. We can consider removing this restriction down the
	// road if it the benefits do not seem to outweigh the disadvantages.
}

// NewPrinter returns a Printer that formats messages tailored to language t.
func NewPrinter(t language.Tag) *Printer {
	p := &Printer{printer{
		tag: t,
	}}
	p.printer.catContext = defaultCatalog.Context(t, &p.printer)
	return p
}

// Sprint is like fmt.Sprint, but using language-specific formatting.
func (p *Printer) Sprint(a ...interface{}) string {
	p.printer.reset()
	p.printer.doPrint(a)
	return p.printer.String()
}

// Fprint is like fmt.Fprint, but using language-specific formatting.
func (p *Printer) Fprint(w io.Writer, a ...interface{}) (n int, err error) {
	p.printer.reset()
	p.printer.doPrint(a)
	n64, err := io.Copy(w, &p.printer.Buffer)
	return int(n64), err
}

// Print is like fmt.Print, but using language-specific formatting.
func (p *Printer) Print(a ...interface{}) (n int, err error) {
	return p.Fprint(os.Stdout, a...)
}

// Sprintln is like fmt.Sprintln, but using language-specific formatting.
func (p *Printer) Sprintln(a ...interface{}) string {
	p.printer.reset()
	p.printer.doPrintln(a)
	return p.printer.String()
}

// Fprintln is like fmt.Fprintln, but using language-specific formatting.
func (p *Printer) Fprintln(w io.Writer, a ...interface{}) (n int, err error) {
	p.printer.reset()
	p.printer.doPrintln(a)
	n64, err := io.Copy(w, &p.printer.Buffer)
	return int(n64), err
}

// Println is like fmt.Println, but using language-specific formatting.
func (p *Printer) Println(a ...interface{}) (n int, err error) {
	return p.Fprintln(os.Stdout, a...)
}

// Sprintf is like fmt.Sprintf, but using language-specific formatting.
func (p *Printer) Sprintf(key Reference, a ...interface{}) string {
	lookupAndFormat(p, key, a)
	return p.printer.String()
}

// Fprintf is like fmt.Fprintf, but using language-specific formatting.
func (p *Printer) Fprintf(w io.Writer, key Reference, a ...interface{}) (n int, err error) {
	lookupAndFormat(p, key, a)
	return w.Write(p.printer.Bytes())
}

// Printf is like fmt.Printf, but using language-specific formatting.
func (p *Printer) Printf(key Reference, a ...interface{}) (n int, err error) {
	lookupAndFormat(p, key, a)
	return os.Stdout.Write(p.printer.Bytes())
}

func lookupAndFormat(p *Printer, r Reference, a []interface{}) {
	p.printer.reset()
	p.printer.args = a
	var id, msg string
	switch v := r.(type) {
	case string:
		id, msg = v, v
	case key:
		id, msg = v.id, v.fallback
	default:
		panic("key argument is not a Reference")
	}

	if p.printer.catContext.Execute(id) == catalog.ErrNotFound {
		if p.printer.catContext.Execute(msg) == catalog.ErrNotFound {
			p.printer.Render(msg)
			return
		}
	}
}

// Arg implements catmsg.Renderer.
func (p *printer) Arg(i int) interface{} { // TODO, also return "ok" bool
	if uint(i) < uint(len(p.args)) {
		return p.args[i]
	}
	return nil
}

// Render implements catmsg.Renderer.
func (p *printer) Render(msg string) {
	p.doPrintf(msg)
}

// A Reference is a string or a message reference.
type Reference interface {
	// TODO: also allow []string
}

// Key creates a message Reference for a message where the given id is used for
// message lookup and the fallback is returned when no matches are found.
func Key(id string, fallback string) Reference {
	return key{id, fallback}
}

type key struct {
	id, fallback string
}
