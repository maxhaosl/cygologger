/*
 * CYGoLogger License
 * -----------
 *
 * CYGoLogger is licensed under the terms of the MIT license reproduced below.
 * This means that CYGoLogger is free software and can be used for both academic
 * and commercial purposes at absolutely no cost.
 *
 * ===============================================================================
 *
 * Copyright (C) 2023-2024 ShiLiang.Hao <newhaosl@163.com>, foobra<vipgs99@gmail.com>
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

// Package Encryption provides a pluggable log-content encryption framework
// (Go port of the C++ CYBaseEncryption / CYEncryptionFactory modules).
//
// In the original C++ library these classes are empty shells (no algorithms
// implemented). This Go port keeps the same extensible shape — an IEncryption
// interface plus a factory — and additionally ships a working AES-GCM
// implementation as a low-risk, ready-to-use example.
package Encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
	"sync"
)

// EEncryptionType enumerates the supported encryption implementations.
type EEncryptionType int

const (
	EncryptionTypeNone EEncryptionType = iota // no-op (pass-through)
	EncryptionTypeAESGCM                      // AES-256-GCM
)

// IEncryption is the pluggable encryption interface. Implementations transform
// plaintext log bytes to ciphertext and back.
type IEncryption interface {
	// Init prepares the algorithm with a key (interpretation is impl-specific).
	Init(key []byte) error
	// Encrypt returns the ciphertext for plain.
	Encrypt(plain []byte) ([]byte, error)
	// Decrypt returns the plaintext for cipherText.
	Decrypt(cipherText []byte) ([]byte, error)
	// GetType returns the implementation type.
	GetType() EEncryptionType
}

// ============================================================================
// CYBaseEncryption - shared base (Go port of C++ CYBaseEncryption shell)
// ============================================================================

type CYBaseEncryption struct {
	encType EEncryptionType
}

func (b *CYBaseEncryption) GetType() EEncryptionType { return b.encType }

// ============================================================================
// CYNoneEncryption - pass-through (matches the C++ "empty shell" behavior)
// ============================================================================

type CYNoneEncryption struct {
	CYBaseEncryption
}

// NewCYNoneEncryption creates a no-op encryptor.
func NewCYNoneEncryption() *CYNoneEncryption {
	e := &CYNoneEncryption{}
	e.encType = EncryptionTypeNone
	return e
}

func (e *CYNoneEncryption) Init(key []byte) error              { return nil }
func (e *CYNoneEncryption) Encrypt(plain []byte) ([]byte, error) { return plain, nil }
func (e *CYNoneEncryption) Decrypt(c []byte) ([]byte, error)     { return c, nil }

// ============================================================================
// CYAESGCMEncryption - AES-256-GCM authenticated encryption (example impl)
// ============================================================================

type CYAESGCMEncryption struct {
	CYBaseEncryption
	mu   sync.Mutex
	gcm  cipher.AEAD
	init bool
}

// NewCYAESGCMEncryption creates an AES-GCM encryptor (call Init before use).
func NewCYAESGCMEncryption() *CYAESGCMEncryption {
	e := &CYAESGCMEncryption{}
	e.encType = EncryptionTypeAESGCM
	return e
}

// Init derives a 256-bit AES key from the provided key material using SHA-256,
// so any non-empty key length is accepted.
func (e *CYAESGCMEncryption) Init(key []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(key) == 0 {
		return fmt.Errorf("encryption: empty key")
	}
	sum := sha256.Sum256(key) // 32-byte key => AES-256
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return fmt.Errorf("encryption: aes cipher failed: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("encryption: gcm failed: %w", err)
	}
	e.gcm = gcm
	e.init = true
	return nil
}

// Encrypt returns nonce||ciphertext (nonce prepended for self-contained decrypt).
func (e *CYAESGCMEncryption) Encrypt(plain []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.init {
		return nil, fmt.Errorf("encryption: not initialized")
	}
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("encryption: nonce failed: %w", err)
	}
	// Seal appends ciphertext to nonce, so the result is nonce||ciphertext||tag.
	return e.gcm.Seal(nonce, nonce, plain, nil), nil
}

// Decrypt reverses Encrypt, expecting nonce||ciphertext input.
func (e *CYAESGCMEncryption) Decrypt(cipherText []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.init {
		return nil, fmt.Errorf("encryption: not initialized")
	}
	ns := e.gcm.NonceSize()
	if len(cipherText) < ns {
		return nil, fmt.Errorf("encryption: ciphertext too short")
	}
	nonce, ct := cipherText[:ns], cipherText[ns:]
	plain, err := e.gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("encryption: open failed: %w", err)
	}
	return plain, nil
}

// ============================================================================
// CYEncryptionFactory - creates encryptors by type (Go port of C++ factory)
// ============================================================================

// CYEncryptionFactory is a singleton factory producing IEncryption implementations.
type CYEncryptionFactory struct {
	mu sync.Mutex
}

var (
	g_CYEncryptionFactoryInstance *CYEncryptionFactory
	g_CYEncryptionFactoryOnce     sync.Once
)

// GetCYEncryptionFactoryInstance returns the singleton encryption factory.
func GetCYEncryptionFactoryInstance() *CYEncryptionFactory {
	g_CYEncryptionFactoryOnce.Do(func() {
		g_CYEncryptionFactoryInstance = &CYEncryptionFactory{}
	})
	return g_CYEncryptionFactoryInstance
}

// CreateEncryption constructs a new encryptor for the given type.
func (f *CYEncryptionFactory) CreateEncryption(eType EEncryptionType) (IEncryption, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch eType {
	case EncryptionTypeNone:
		return NewCYNoneEncryption(), nil
	case EncryptionTypeAESGCM:
		return NewCYAESGCMEncryption(), nil
	default:
		return nil, fmt.Errorf("unsupported encryption type: %d", eType)
	}
}

// Compile-time interface checks.
var (
	_ IEncryption = (*CYNoneEncryption)(nil)
	_ IEncryption = (*CYAESGCMEncryption)(nil)
)
