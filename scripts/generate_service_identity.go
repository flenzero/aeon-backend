package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"

	"github.com/flenzero/aeon-backend/internal/chain"
)

func main() {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("SERVICE_PUBLIC_KEY=%s\n", chain.EncodeBase58(publicKey))
	fmt.Printf("SERVICE_PRIVATE_KEY=%s\n", chain.EncodeBase58(privateKey))
}
