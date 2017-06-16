package oauth1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewToken(t *testing.T) {
	expectedToken := "token"
	expectedSecret := "secret"
	tk := NewToken(expectedToken, expectedSecret)
	assert.Equal(t, expectedToken, tk.Token)
	assert.Equal(t, expectedSecret, tk.TokenSecret)
}

func TestStaticTokenSource(t *testing.T) {
	ts := StaticTokenSource(NewToken("t", "s"))
	tk, err := ts.Token()
	assert.Nil(t, err)
	assert.Equal(t, "t", tk.Token)
}

func TestStaticTokenSourceEmpty(t *testing.T) {
	ts := StaticTokenSource(nil)
	tk, err := ts.Token()
	assert.Nil(t, tk)
	if assert.Error(t, err) {
		assert.Equal(t, "oauth1: Token is nil", err.Error())
	}
}
