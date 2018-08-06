package gen_test

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

func TestMapOf(t *testing.T) {
	genParams := gopter.DefaultGenParameters()
	genParams.MaxSize = 50
	keyGen := gen.Identifier()
	elementGen := gen.Const("element")
	mapGen := gen.MapOf(keyGen, elementGen)

	for i := 0; i < 100; i++ {
		sample, ok := mapGen(genParams).Retrieve()

		if !ok {
			t.Error("Sample was not ok")
		}
		strings, ok := sample.(map[string]string)
		if !ok {
			t.Errorf("Sample not slice of string: %#v", sample)
		} else {
			if len(strings) > 50 {
				t.Errorf("Sample has invalid length: %#v", len(strings))
			}
			for _, value := range strings {
				if value != "element" {
					t.Errorf("Sample contains invalid value: %#v", sample)
				}
			}
		}
	}

	genParams.MaxSize = 10

	for i := 0; i < 100; i++ {
		sample, ok := mapGen(genParams).Retrieve()

		if !ok {
			t.Error("Sample was not ok")
		}
		strings, ok := sample.(map[string]string)
		if !ok {
			t.Errorf("Sample not slice of string: %#v", sample)
		} else {
			if len(strings) > 10 {
				t.Errorf("Sample has invalid length: %#v", len(strings))
			}
			for _, value := range strings {
				if value != "element" {
					t.Errorf("Sample contains invalid value: %#v", sample)
				}
			}
		}
	}

	genParams.MaxSize = 0
	genParams.MinSize = 0

	for i := 0; i < 100; i++ {
		sample, ok := mapGen(genParams).Retrieve()

		if !ok {
			t.Error("Sample was not ok")
		}
		strings, ok := sample.(map[string]string)
		if !ok {
			t.Errorf("Sample not slice of string: %#v", sample)
		} else {
			if len(strings) != 0 {
				t.Errorf("Sample has invalid length: %#v", len(strings))
			}
		}
	}
}

func TestMapOfPanic(t *testing.T) {
	genParams := gopter.DefaultGenParameters()
	genParams.MaxSize = 0
	genParams.MinSize = 1
	keyGen := gen.Identifier()
	elementGen := gen.Const("element")
	mapGen := gen.MapOf(keyGen, elementGen)

	defer func() {
		if r := recover(); r == nil {
			t.Error("SliceOf did not panic when MinSize was > MaxSize")
		}
	}()

	mapGen(genParams).Retrieve()
}
