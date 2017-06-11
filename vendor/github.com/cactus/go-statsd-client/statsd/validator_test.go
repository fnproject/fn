// Copyright (c) 2012-2016 Eli Janssen
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package statsd

import "testing"

var validatorTests = []struct {
	Stat  string
	Valid bool
}{
	{"test.one", true},
	{"test#two", false},
	{"test|three", false},
	{"test@four", false},
}

func TestValidator(t *testing.T) {
	var err error
	for _, tt := range validatorTests {
		err = CheckName(tt.Stat)
		switch {
		case err != nil && tt.Valid:
			t.Fatal(err)
		case err == nil && !tt.Valid:
			t.Fatalf("validation should have failed for %s", tt.Stat)
		}
	}
}
