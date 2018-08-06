package gen_test

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
)

func TestRune(t *testing.T) {
	commonGeneratorTest(t, "rune", gen.Rune(), func(value interface{}) bool {
		v, ok := value.(rune)
		return ok && utf8.ValidRune(v)
	})
}

func TestNumChar(t *testing.T) {
	commonGeneratorTest(t, "num char", gen.NumChar(), func(value interface{}) bool {
		v, ok := value.(rune)
		return ok && unicode.IsNumber(v)
	})
}

func TestAlphaUpper(t *testing.T) {
	commonGeneratorTest(t, "alpha upper char", gen.AlphaUpperChar(), func(value interface{}) bool {
		v, ok := value.(rune)
		return ok && unicode.IsUpper(v) && unicode.IsLetter(v)
	})
}

func TestAlphaLower(t *testing.T) {
	commonGeneratorTest(t, "alpha lower char", gen.AlphaLowerChar(), func(value interface{}) bool {
		v, ok := value.(rune)
		return ok && unicode.IsLower(v) && unicode.IsLetter(v)
	})
}

func TestAlphaChar(t *testing.T) {
	commonGeneratorTest(t, "alpha char", gen.AlphaChar(), func(value interface{}) bool {
		v, ok := value.(rune)
		return ok && unicode.IsLetter(v)
	})
}

func TestAnyString(t *testing.T) {
	commonGeneratorTest(t, "any string", gen.AnyString(), func(value interface{}) bool {
		str, ok := value.(string)

		if !ok {
			return false
		}
		for _, ch := range str {
			if !utf8.ValidRune(ch) {
				return false
			}
		}
		return true
	})
}

func TestAlphaString(t *testing.T) {
	alphaString := gen.AlphaString()
	commonGeneratorTest(t, "alpha string", alphaString, func(value interface{}) bool {
		str, ok := value.(string)

		if !ok {
			return false
		}
		for _, ch := range str {
			if !utf8.ValidRune(ch) || !unicode.IsLetter(ch) {
				return false
			}
		}
		return true
	})
	sieve := alphaString(gopter.DefaultGenParameters()).Sieve
	if sieve == nil {
		t.Error("No sieve")
	}
	if !sieve("abcdABCD") || sieve("abc12") {
		t.Error("Invalid sieve")
	}
}

func TestNumString(t *testing.T) {
	numString := gen.NumString()
	commonGeneratorTest(t, "num string", numString, func(value interface{}) bool {
		str, ok := value.(string)

		if !ok {
			return false
		}
		for _, ch := range str {
			if !utf8.ValidRune(ch) || !unicode.IsDigit(ch) {
				return false
			}
		}
		return true
	})
	sieve := numString(gopter.DefaultGenParameters()).Sieve
	if sieve == nil {
		t.Error("No sieve")
	}
	if !sieve("123456789") || sieve("123abcd") {
		t.Error("Invalid sieve")
	}
}

func TestIdentifier(t *testing.T) {
	identifiers := gen.Identifier()
	commonGeneratorTest(t, "identifiers", identifiers, func(value interface{}) bool {
		str, ok := value.(string)

		if !ok {
			return false
		}
		if len(str) == 0 || !unicode.IsLetter([]rune(str)[0]) {
			return false
		}
		for _, ch := range str {
			if !utf8.ValidRune(ch) || (!unicode.IsDigit(ch) && !unicode.IsLetter(ch)) {
				return false
			}
		}
		return true
	})
	sieve := identifiers(gopter.DefaultGenParameters()).Sieve
	if sieve == nil {
		t.Error("No sieve")
	}
	if !sieve("abc123") || sieve("123abc") || sieve("abcd123-") {
		t.Error("Invalid sieve")
	}
}

func TestUnicodeString(t *testing.T) {
	fail := gen.UnicodeChar(nil)
	value, ok := fail.Sample()
	if value != nil || ok {
		t.Fail()
	}

	for _, table := range unicode.Scripts {
		unicodeString := gen.UnicodeString(table)
		commonGeneratorTest(t, "unicodeString", unicodeString, func(value interface{}) bool {
			str, ok := value.(string)

			if !ok {
				return false
			}
			for _, ch := range str {
				if !utf8.ValidRune(ch) || !unicode.Is(table, ch) {
					return false
				}
			}
			return true
		})
	}
}
