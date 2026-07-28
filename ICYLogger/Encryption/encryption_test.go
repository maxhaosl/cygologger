package Encryption

import (
	"bytes"
	"testing"
)

// TestAESGCMRoundTrip verifies Encrypt/Decrypt round-trips arbitrary payloads.
func TestAESGCMRoundTrip(t *testing.T) {
	enc := NewCYAESGCMEncryption()
	if err := enc.Init([]byte("unit-test-key")); err != nil {
		t.Fatalf("Init: %v", err)
	}
	payloads := [][]byte{
		[]byte(""),
		[]byte("hello log line"),
		bytes.Repeat([]byte{0x00, 0xff, 0x7f}, 512),
	}
	for _, plain := range payloads {
		ct, err := enc.Encrypt(plain)
		if err != nil {
			t.Fatalf("Encrypt(%d bytes): %v", len(plain), err)
		}
		if len(plain) > 0 && bytes.Contains(ct, plain) {
			t.Errorf("ciphertext must not contain plaintext")
		}
		got, err := enc.Decrypt(ct)
		if err != nil {
			t.Fatalf("Decrypt: %v", err)
		}
		if !bytes.Equal(got, plain) {
			t.Errorf("round-trip mismatch: got %d bytes, want %d bytes", len(got), len(plain))
		}
	}
}

// TestAESGCMNonceUniqueness verifies two encryptions of the same plaintext
// produce different ciphertexts (random nonce).
func TestAESGCMNonceUniqueness(t *testing.T) {
	enc := NewCYAESGCMEncryption()
	if err := enc.Init([]byte("key")); err != nil {
		t.Fatalf("Init: %v", err)
	}
	c1, _ := enc.Encrypt([]byte("same message"))
	c2, _ := enc.Encrypt([]byte("same message"))
	if bytes.Equal(c1, c2) {
		t.Errorf("two encryptions produced identical ciphertext; nonce not random")
	}
}

// TestAESGCMTamperDetection verifies GCM authentication rejects modified data.
func TestAESGCMTamperDetection(t *testing.T) {
	enc := NewCYAESGCMEncryption()
	if err := enc.Init([]byte("key")); err != nil {
		t.Fatalf("Init: %v", err)
	}
	ct, err := enc.Encrypt([]byte("sensitive"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	ct[len(ct)-1] ^= 0x01
	if _, err := enc.Decrypt(ct); err == nil {
		t.Errorf("Decrypt of tampered ciphertext must fail")
	}
}

// TestAESGCMWrongKeyFails verifies decryption with a different key fails.
func TestAESGCMWrongKeyFails(t *testing.T) {
	enc1 := NewCYAESGCMEncryption()
	enc2 := NewCYAESGCMEncryption()
	if err := enc1.Init([]byte("key-A")); err != nil {
		t.Fatalf("Init A: %v", err)
	}
	if err := enc2.Init([]byte("key-B")); err != nil {
		t.Fatalf("Init B: %v", err)
	}
	ct, _ := enc1.Encrypt([]byte("secret"))
	if _, err := enc2.Decrypt(ct); err == nil {
		t.Errorf("Decrypt with wrong key must fail")
	}
}

// TestAESGCMErrors verifies error handling: empty key, use before Init,
// truncated ciphertext.
func TestAESGCMErrors(t *testing.T) {
	enc := NewCYAESGCMEncryption()
	if err := enc.Init(nil); err == nil {
		t.Errorf("Init with empty key must fail")
	}
	if _, err := enc.Encrypt([]byte("x")); err == nil {
		t.Errorf("Encrypt before successful Init must fail")
	}
	if _, err := enc.Decrypt([]byte("x")); err == nil {
		t.Errorf("Decrypt before successful Init must fail")
	}
	if err := enc.Init([]byte("k")); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if _, err := enc.Decrypt([]byte{0x01, 0x02}); err == nil {
		t.Errorf("Decrypt of too-short ciphertext must fail")
	}
}

// TestNoneEncryptionPassThrough verifies the no-op encryptor returns input as-is.
func TestNoneEncryptionPassThrough(t *testing.T) {
	enc := NewCYNoneEncryption()
	if err := enc.Init(nil); err != nil {
		t.Fatalf("Init: %v", err)
	}
	in := []byte("plain")
	ct, err := enc.Encrypt(in)
	if err != nil || !bytes.Equal(ct, in) {
		t.Errorf("Encrypt = %q, %v; want pass-through", ct, err)
	}
	pt, err := enc.Decrypt(in)
	if err != nil || !bytes.Equal(pt, in) {
		t.Errorf("Decrypt = %q, %v; want pass-through", pt, err)
	}
	if enc.GetType() != EncryptionTypeNone {
		t.Errorf("GetType = %v, want EncryptionTypeNone", enc.GetType())
	}
}

// TestEncryptionFactory verifies singleton identity, type dispatch, and the
// unsupported-type error path.
func TestEncryptionFactory(t *testing.T) {
	f := GetCYEncryptionFactoryInstance()
	if f == nil || f != GetCYEncryptionFactoryInstance() {
		t.Fatal("factory singleton is not stable")
	}

	none, err := f.CreateEncryption(EncryptionTypeNone)
	if err != nil || none.GetType() != EncryptionTypeNone {
		t.Errorf("CreateEncryption(None) = %v, %v", none, err)
	}
	aes, err := f.CreateEncryption(EncryptionTypeAESGCM)
	if err != nil || aes.GetType() != EncryptionTypeAESGCM {
		t.Errorf("CreateEncryption(AESGCM) = %v, %v", aes, err)
	}
	if _, err := f.CreateEncryption(EEncryptionType(999)); err == nil {
		t.Errorf("CreateEncryption(999) must fail")
	}
}
