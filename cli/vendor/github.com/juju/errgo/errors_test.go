// Copyright 2014 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package errgo_test

import (
	"fmt"
	"io/ioutil"
	"runtime"
	"strings"
	"testing"

	gc "gopkg.in/check.v1"

	"github.com/juju/errgo"
)

var (
	_ errgo.Wrapper    = (*errgo.Err)(nil)
	_ errgo.Locationer = (*errgo.Err)(nil)
	_ errgo.Causer     = (*errgo.Err)(nil)
)

func Test(t *testing.T) {
	gc.TestingT(t)
}

type errorsSuite struct{}

var _ = gc.Suite(&errorsSuite{})

func (*errorsSuite) TestNew(c *gc.C) {
	err := errgo.New("foo") //err TestNew
	checkErr(c, err, nil, "foo", "[{$TestNew$: foo}]", err)
}

func (*errorsSuite) TestNewf(c *gc.C) {
	err := errgo.Newf("foo %d", 5) //err TestNewf
	checkErr(c, err, nil, "foo 5", "[{$TestNewf$: foo 5}]", err)
}

var someErr = errgo.New("some error") //err varSomeErr

func annotate1() error {
	err := errgo.Notef(someErr, "annotate1") //err annotate1
	return err
}

func annotate2() error {
	err := annotate1()
	err = errgo.Notef(err, "annotate2") //err annotate2
	return err
}

func (*errorsSuite) TestNoteUsage(c *gc.C) {
	err0 := annotate2()
	err, ok := err0.(errgo.Wrapper)
	c.Assert(ok, gc.Equals, true)
	underlying := err.Underlying()
	checkErr(
		c, err0, underlying,
		"annotate2: annotate1: some error",
		"[{$annotate2$: annotate2} {$annotate1$: annotate1} {$varSomeErr$: some error}]",
		err0)
}

func (*errorsSuite) TestMask(c *gc.C) {
	err0 := errgo.WithCausef(nil, someErr, "foo") //err TestMask#0
	err := errgo.Mask(err0)                       //err TestMask#1
	checkErr(c, err, err0, "foo", "[{$TestMask#1$: } {$TestMask#0$: foo}]", err)

	err = errgo.Mask(nil)
	c.Assert(err, gc.IsNil)
}

func (*errorsSuite) TestNotef(c *gc.C) {
	err0 := errgo.WithCausef(nil, someErr, "foo") //err TestNotef#0
	err := errgo.Notef(err0, "bar")               //err TestNotef#1
	checkErr(c, err, err0, "bar: foo", "[{$TestNotef#1$: bar} {$TestNotef#0$: foo}]", err)

	err = errgo.Notef(nil, "bar") //err TestNotef#2
	checkErr(c, err, nil, "bar", "[{$TestNotef#2$: bar}]", err)
}

func (*errorsSuite) TestMaskFunc(c *gc.C) {
	err0 := errgo.New("zero")
	err1 := errgo.New("one")

	allowVals := func(vals ...error) (r []func(error) bool) {
		for _, val := range vals {
			r = append(r, errgo.Is(val))
		}
		return
	}
	tests := []struct {
		err    error
		allow0 []func(error) bool
		allow1 []func(error) bool
		cause  error
	}{{
		err:    err0,
		allow0: allowVals(err0),
		cause:  err0,
	}, {
		err:    err1,
		allow0: allowVals(err0),
		cause:  nil,
	}, {
		err:    err0,
		allow1: allowVals(err0),
		cause:  err0,
	}, {
		err:    err0,
		allow0: allowVals(err1),
		allow1: allowVals(err0),
		cause:  err0,
	}, {
		err:    err0,
		allow0: allowVals(err0, err1),
		cause:  err0,
	}, {
		err:    err1,
		allow0: allowVals(err0, err1),
		cause:  err1,
	}, {
		err:    err0,
		allow1: allowVals(err0, err1),
		cause:  err0,
	}, {
		err:    err1,
		allow1: allowVals(err0, err1),
		cause:  err1,
	}}
	for i, test := range tests {
		c.Logf("test %d", i)
		wrap := errgo.MaskFunc(test.allow0...)
		err := wrap(test.err, test.allow1...)
		cause := errgo.Cause(err)
		wantCause := test.cause
		if wantCause == nil {
			wantCause = err
		}
		c.Check(cause, gc.Equals, wantCause)
	}
}

type embed struct {
	*errgo.Err
}

func (*errorsSuite) TestCause(c *gc.C) {
	c.Assert(errgo.Cause(someErr), gc.Equals, someErr)

	causeErr := errgo.New("cause error")
	underlyingErr := errgo.New("underlying error")                 //err TestCause#1
	err := errgo.WithCausef(underlyingErr, causeErr, "foo %d", 99) //err TestCause#2
	c.Assert(errgo.Cause(err), gc.Equals, causeErr)

	checkErr(c, err, underlyingErr, "foo 99: underlying error", "[{$TestCause#2$: foo 99} {$TestCause#1$: underlying error}]", causeErr)

	err = &embed{err.(*errgo.Err)}
	c.Assert(errgo.Cause(err), gc.Equals, causeErr)
}

func (*errorsSuite) TestDetails(c *gc.C) {
	c.Assert(errgo.Details(nil), gc.Equals, "[]")

	otherErr := fmt.Errorf("other")
	checkErr(c, otherErr, nil, "other", "[{other}]", otherErr)

	err0 := &embed{errgo.New("foo").(*errgo.Err)} //err TestStack#0
	checkErr(c, err0, nil, "foo", "[{$TestStack#0$: foo}]", err0)

	err1 := &embed{errgo.Notef(err0, "bar").(*errgo.Err)} //err TestStack#1
	checkErr(c, err1, err0, "bar: foo", "[{$TestStack#1$: bar} {$TestStack#0$: foo}]", err1)

	err2 := errgo.Mask(err1) //err TestStack#2
	checkErr(c, err2, err1, "bar: foo", "[{$TestStack#2$: } {$TestStack#1$: bar} {$TestStack#0$: foo}]", err2)
}

func (*errorsSuite) TestMatch(c *gc.C) {
	type errTest func(error) bool
	allow := func(ss ...string) []func(error) bool {
		fns := make([]func(error) bool, len(ss))
		for i, s := range ss {
			s := s
			fns[i] = func(err error) bool {
				return err != nil && err.Error() == s
			}
		}
		return fns
	}
	tests := []struct {
		err error
		fns []func(error) bool
		ok  bool
	}{{
		err: errgo.New("foo"),
		fns: allow("foo"),
		ok:  true,
	}, {
		err: errgo.New("foo"),
		fns: allow("bar"),
		ok:  false,
	}, {
		err: errgo.New("foo"),
		fns: allow("bar", "foo"),
		ok:  true,
	}, {
		err: errgo.New("foo"),
		fns: nil,
		ok:  false,
	}, {
		err: nil,
		fns: nil,
		ok:  false,
	}}

	for i, test := range tests {
		c.Logf("test %d", i)
		c.Assert(errgo.Match(test.err, test.fns...), gc.Equals, test.ok)
	}
}

func (*errorsSuite) TestLocation(c *gc.C) {
	loc := errgo.Location{File: "foo", Line: 35}
	c.Assert(loc.String(), gc.Equals, "foo:35")
}

func checkErr(c *gc.C, err, underlying error, msg string, details string, cause error) {
	c.Assert(err, gc.NotNil)
	c.Assert(err.Error(), gc.Equals, msg)
	if err, ok := err.(errgo.Wrapper); ok {
		c.Assert(err.Underlying(), gc.Equals, underlying)
	} else {
		c.Assert(underlying, gc.IsNil)
	}
	c.Assert(errgo.Cause(err), gc.Equals, cause)
	wantDetails := replaceLocations(details)
	c.Assert(errgo.Details(err), gc.Equals, wantDetails)
}

func replaceLocations(s string) string {
	t := ""
	for {
		i := strings.Index(s, "$")
		if i == -1 {
			break
		}
		t += s[0:i]
		s = s[i+1:]
		i = strings.Index(s, "$")
		if i == -1 {
			panic("no second $")
		}
		t += location(s[0:i]).String()
		s = s[i+1:]
	}
	t += s
	return t
}

func location(tag string) errgo.Location {
	line, ok := tagToLine[tag]
	if !ok {
		panic(fmt.Errorf("tag %q not found", tag))
	}
	return errgo.Location{
		File: filename,
		Line: line,
	}
}

var tagToLine = make(map[string]int)
var filename string

func init() {
	data, err := ioutil.ReadFile("errors_test.go")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	for i, line := range lines {
		if j := strings.Index(line, "//err "); j >= 0 {
			tagToLine[line[j+len("//err "):]] = i + 1
		}
	}
	_, filename, _, _ = runtime.Caller(0)
}
