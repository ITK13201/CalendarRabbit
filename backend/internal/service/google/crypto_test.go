package google_test

import (
	"testing"

	"github.com/ITK13201/CalendarRabbit/backend/internal/service/google"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCipher_RoundTrip(t *testing.T) {
	c, err := google.NewCipher("test-encryption-key")
	require.NoError(t, err)

	plaintext := "1//refresh-token-value"
	enc, err := c.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, enc)

	dec, err := c.Decrypt(enc)
	require.NoError(t, err)
	assert.Equal(t, plaintext, dec)
}

func TestCipher_DifferentNoncePerEncrypt(t *testing.T) {
	c, err := google.NewCipher("key")
	require.NoError(t, err)

	a, err := c.Encrypt("same")
	require.NoError(t, err)
	b, err := c.Encrypt("same")
	require.NoError(t, err)
	// GCM は毎回ランダム nonce のため暗号文は一致しない。
	assert.NotEqual(t, a, b)
}

func TestCipher_WrongKeyFails(t *testing.T) {
	enc, err := mustCipher(t, "correct-key").Encrypt("secret")
	require.NoError(t, err)

	_, err = mustCipher(t, "wrong-key").Decrypt(enc)
	require.Error(t, err)
}

func TestCipher_EmptyKey(t *testing.T) {
	_, err := google.NewCipher("")
	assert.ErrorIs(t, err, google.ErrEmptyEncKey)
}

func mustCipher(t *testing.T, key string) *google.Cipher {
	t.Helper()
	c, err := google.NewCipher(key)
	require.NoError(t, err)
	return c
}
