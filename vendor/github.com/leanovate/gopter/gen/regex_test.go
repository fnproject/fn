package gen_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/leanovate/gopter/gen"
)

func TestRegexMatch(t *testing.T) {
	regexs := []string{
		"[a-z][0-9a-zA-Z]*",
		"AB[0-9]+",
		"1?(zero|one)0",
		"ABCD.+1234",
		"^[0-9]{3}[A-Z]{5,}[a-z]{10,20}$",
		"(?s)[^0-9]*ABCD.*1234",
	}
	for _, regex := range regexs {
		pattern, err := regexp.Compile(regex)
		if err != nil {
			t.Error("Invalid regex", err)
		}
		commonGeneratorTest(t, fmt.Sprintf("matches for %s", regex), gen.RegexMatch(regex), func(value interface{}) bool {
			str, ok := value.(string)
			return ok && pattern.MatchString(str)
		})
	}

	gen := gen.RegexMatch("]]}})Invalid{]]]")
	value, ok := gen.Sample()
	if ok || value != nil {
		t.Errorf("Invalid value: %#v", value)
	}
}
