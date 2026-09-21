// Package google は Google Calendar 連携（OAuth・専用カレンダー操作・トークン暗号化）を提供する。
package google

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrEmptyEncKey は暗号鍵が未設定のときに返す。
var ErrEmptyEncKey = errors.New("google: token encryption key is empty")

// Cipher は refresh token をアプリ層で対称暗号（AES-256-GCM）する。
// 鍵は任意長の文字列を SHA-256 で 32byte に畳んで AES-256 鍵として用いる（design.md D3b）。
type Cipher struct {
	key []byte
}

// NewCipher は暗号鍵文字列から Cipher を生成する。鍵が空なら ErrEmptyEncKey を返す。
func NewCipher(encKey string) (*Cipher, error) {
	if encKey == "" {
		return nil, ErrEmptyEncKey
	}
	sum := sha256.Sum256([]byte(encKey))
	return &Cipher{key: sum[:]}, nil
}

// Encrypt は平文を暗号化し base64(nonce||ciphertext) を返す。
func (c *Cipher) Encrypt(plaintext string) (string, error) {
	gcm, err := c.newGCM()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("google: read nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt は Encrypt の出力を復号する。鍵不一致・改竄時はエラーを返す。
func (c *Cipher) Decrypt(encoded string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("google: decode base64: %w", err)
	}
	gcm, err := c.newGCM()
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return "", errors.New("google: ciphertext too short")
	}
	nonce, ciphertext := raw[:nonceSize], raw[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("google: decrypt: %w", err)
	}
	return string(plaintext), nil
}

func (c *Cipher) newGCM() (cipher.AEAD, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return nil, fmt.Errorf("google: new cipher: %w", err)
	}
	return cipher.NewGCM(block)
}
