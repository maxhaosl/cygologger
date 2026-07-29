// Command new_features demonstrates the newly ported cygologger capabilities:
//   - One-line initialization + Go-idiomatic logging
//   - Hex dump logging
//   - Escape (special-character-safe) logging
//   - Crash/exception logging via panic-recover
//   - AES-GCM log-content encryption
//   - (FTP upload API — shown but not executed unless a server is available)
package main

import (
	"bytes"
	"fmt"

	gologger "github.com/maxhaosl/cygologger/ICYLogger"
)

func main() {
	fmt.Println("=== cygologger New Features Example ===")

	// ---- 1. One-line init (console + files under ./logs) ----
	gologger.InitDefault("./logs")
	defer gologger.Close()

	// Enable crash logging (writes ./logs/Exception.log).
	gologger.InitException("./logs")

	// ---- 2. Standard Go-idiomatic logging (auto file/line/func capture) ----
	gologger.Info("service started, version=%s", "1.0.0")
	gologger.Debug("debug value = %d", 42)
	gologger.Warn("this is a warning")

	// ---- 3. Hex dump logging ----
	payload := []byte("Hello, cygologger! \x00\x01\x02binary")
	gologger.HexInfo(payload)

	// ---- 4. Escape logging (safely logs brackets/commas) ----
	gologger.EscapeInfo("value with ]brackets, and commas that need escaping")

	// Hex/Escape variants for Main/Remote/Sys types:
	gologger.HexMain([]byte("hex to Main log"))
	gologger.EscapeMain("escape to Main log with ]chars")
	gologger.SetWriteRemote(true)
	gologger.HexRemote([]byte("hex to Remote log"))
	gologger.EscapeRemote("escape to Remote log with ]chars")
	gologger.SetWriteRemote(false)

	// ---- 5. Crash / exception handling ----
	demoRecover()
	gologger.SafeGo(func() {
		panic("boom in a background goroutine")
	})

	// ---- 6. AES-GCM encryption of sensitive content ----
	demoEncryption()

	// ---- 7. FTP upload API (compile-time demo only) ----
	demoUploadAPI()

	// Give the SafeGo goroutine a moment and flush everything.
	gologger.Flush()
	fmt.Println("=== Example Complete (see ./logs for output) ===")
}

func demoRecover() {
	defer gologger.Recover() // captures the panic below, logs it, then continues
	panic("intentional panic for demonstration")
}

func demoEncryption() {
	enc, err := gologger.NewAESEncryptor([]byte("my-super-secret-key"))
	if err != nil {
		gologger.Error("create encryptor failed: %v", err)
		return
	}
	plain := []byte("sensitive log line: user=admin token=abc123")
	cipherText, err := enc.Encrypt(plain)
	if err != nil {
		gologger.Error("encrypt failed: %v", err)
		return
	}
	decrypted, err := enc.Decrypt(cipherText)
	if err != nil {
		gologger.Error("decrypt failed: %v", err)
		return
	}
	if bytes.Equal(plain, decrypted) {
		gologger.Info("AES-GCM round-trip OK (cipher len=%d)", len(cipherText))
		fmt.Printf("  encryption round-trip OK, cipher bytes=%d\n", len(cipherText))
	} else {
		gologger.Error("AES-GCM round-trip MISMATCH")
	}
}

func demoUploadAPI() {
	// This only demonstrates the API shape. To actually upload, point cfg at a
	// reachable FTP server and uncomment the call below.
	cfg := &gologger.UpLoadConfig{
		Host:      "127.0.0.1",
		Port:      21,
		User:      "user",
		Password:  "pass",
		RemoteDir: "/logs",
		Passive:   true,
	}
	_ = cfg
	// if err := gologger.UploadLogFTP(cfg, "./logs/Info.log", ""); err != nil {
	//     gologger.Error("ftp upload failed: %v", err)
	// }
	fmt.Println("  FTP upload API ready (see demoUploadAPI for usage)")
}
