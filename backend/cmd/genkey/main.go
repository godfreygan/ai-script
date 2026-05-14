package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func main() {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("generate key failed: %v", err))
	}

	base64Key := base64.StdEncoding.EncodeToString(b)

	fmt.Println("# 生成的 AES-256 密钥")
	fmt.Println("# 把这行完整复制到你的 .env 或环境变量中即可:")
	fmt.Printf("CRYPTO_KEY_BASE64=%s\n", base64Key)
}
