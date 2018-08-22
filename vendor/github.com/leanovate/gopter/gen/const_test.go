package gen_test

import (
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestConstGen(t *testing.T) {
	commonGeneratorTest(t, "const", gen.Const("some constant"), func(value interface{}) bool {
		v, ok := value.(string)
		return ok && v == "some constant"
	})
}
