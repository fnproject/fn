package encryption

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncryptDecrypt(t *testing.T) {
	// not providing an encrypter will fail
	msg := []byte("hello again swarmkit")
	_, err := Encrypt(msg, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no encrypter")

	// noop encrypter can encrypt
	encrypted, err := Encrypt(msg, NoopCrypter)
	require.NoError(t, err)

	// not providing a decrypter will fail
	_, err = Decrypt(encrypted, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no decrypter")

	// noop decrypter can decrypt
	decrypted, err := Decrypt(encrypted, NoopCrypter)
	require.NoError(t, err)
	require.Equal(t, msg, decrypted)

	// the default encrypter can produce something the default decrypter can read
	encrypter, decrypter := Defaults([]byte("key"))
	encrypted, err = Encrypt(msg, encrypter)
	require.NoError(t, err)
	decrypted, err = Decrypt(encrypted, decrypter)
	require.NoError(t, err)
	require.Equal(t, msg, decrypted)

	// mismatched encrypters and decrypters can't read the content produced by each
	encrypted, err = Encrypt(msg, NoopCrypter)
	require.NoError(t, err)
	_, err = Decrypt(encrypted, decrypter)
	require.Error(t, err)
	require.IsType(t, ErrCannotDecrypt{}, err)

	encrypted, err = Encrypt(msg, encrypter)
	require.NoError(t, err)
	_, err = Decrypt(encrypted, NoopCrypter)
	require.Error(t, err)
	require.IsType(t, ErrCannotDecrypt{}, err)
}

func TestHumanReadable(t *testing.T) {
	// we can produce human readable strings that can then be re-parsed
	key := GenerateSecretKey()
	keyString := HumanReadableKey(key)
	parsedKey, err := ParseHumanReadableKey(keyString)
	require.NoError(t, err)
	require.Equal(t, parsedKey, key)

	// if the prefix is wrong, we can't parse the key
	_, err = ParseHumanReadableKey("A" + keyString)
	require.Error(t, err)

	// With the right prefix, we can't parse if the key isn't base64 encoded
	_, err = ParseHumanReadableKey(humanReadablePrefix + "aaa*aa/")
	require.Error(t, err)

	// Extra padding also fails
	_, err = ParseHumanReadableKey(keyString + "=")
	require.Error(t, err)
}
