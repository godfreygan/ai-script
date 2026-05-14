package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// AES-256-GCM 加解密,用于保存模型 api_key 等敏感信息

type Cipher struct {
	gcm cipher.AEAD
}

func New(b64Key string) (*Cipher, error) {
	key, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		return nil, err
	}
	return newFromBytes(key)
}

// NewFromBytes 使用原始 32 字节密钥构造 Cipher。
func NewFromBytes(key []byte) (*Cipher, error) {
	return newFromBytes(key)
}

func newFromBytes(key []byte) (*Cipher, error) {
	if len(key) != 32 {
		return nil, errors.New("crypto key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{gcm: gcm}, nil
}

func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return c.gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	ns := c.gcm.NonceSize()
	if len(ciphertext) < ns {
		return nil, errors.New("ciphertext too short")
	}
	nonce, ct := ciphertext[:ns], ciphertext[ns:]
	return c.gcm.Open(nil, nonce, ct, nil)
}
