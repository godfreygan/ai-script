package crypto

import (
	"encoding/base64"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	b64Key := base64.StdEncoding.EncodeToString(key)

	cipher, err := New(b64Key)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	plaintext := []byte("hello world, this is a secret message")
	ciphertext, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}
	if len(ciphertext) == 0 {
		t.Fatal("ciphertext is empty")
	}

	decrypted, err := cipher.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1

	c1, err := New(base64.StdEncoding.EncodeToString(key1))
	if err != nil {
		t.Fatalf("New c1 failed: %v", err)
	}
	c2, err := New(base64.StdEncoding.EncodeToString(key2))
	if err != nil {
		t.Fatalf("New c2 failed: %v", err)
	}

	plaintext := []byte("sensitive data")
	ciphertext, err := c1.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	_, err = c2.Decrypt(ciphertext)
	if err == nil {
		t.Fatal("expected error decrypting with wrong key, got nil")
	}
}

func TestNewInvalidKeyLength(t *testing.T) {
	// 16 bytes base64 -> 12 raw bytes, not 32
	_, err := New(base64.StdEncoding.EncodeToString(make([]byte, 12)))
	if err == nil {
		t.Fatal("expected error for invalid key length, got nil")
	}
}

func TestNewInvalidBase64(t *testing.T) {
	_, err := New("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestDecryptCiphertextTooShort(t *testing.T) {
	key := make([]byte, 32)
	cipher, err := New(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	_, err = cipher.Decrypt([]byte{0x01, 0x02})
	if err == nil {
		t.Fatal("expected error for short ciphertext, got nil")
	}
}

func TestEncryptProducesDifferentCiphertext(t *testing.T) {
	key := make([]byte, 32)
	cipher, err := New(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	plaintext := []byte("deterministic plaintext")
	ct1, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1 failed: %v", err)
	}
	ct2, err := cipher.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2 failed: %v", err)
	}

	// AES-GCM uses random nonce, so ciphertexts should differ
	match := true
	if len(ct1) != len(ct2) {
		match = false
	} else {
		for i := range ct1 {
			if ct1[i] != ct2[i] {
				match = false
				break
			}
		}
	}
	if match {
		t.Fatal("two encryptions of same plaintext produced identical ciphertext")
	}
}
