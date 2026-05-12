package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
)

func main() {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	fmt.Println(base64.StdEncoding.EncodeToString(b))
}
