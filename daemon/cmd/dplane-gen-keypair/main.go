package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
)

func main() {
	outDir := flag.String("out", ".", "Output directory for keys")
	flag.Parse()

	fmt.Println("Generating Ed25519 keypair for license signing...")

	pubKey, privKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate keypair: %v\n", err)
		os.Exit(1)
	}

	pubKeyB64 := base64.StdEncoding.EncodeToString(pubKey)
	privKeyB64 := base64.StdEncoding.EncodeToString(privKey)

	privKeyFile := fmt.Sprintf("%s/license.key.private", *outDir)
	pubKeyFile := fmt.Sprintf("%s/license.key.public", *outDir)

	if err := os.WriteFile(privKeyFile, []byte(privKeyB64), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write private key: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(pubKeyFile, []byte(pubKeyB64), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write public key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Keys generated:\n")
	fmt.Printf("  Private key: %s (chmod 600)\n", privKeyFile)
	fmt.Printf("  Public key:  %s\n\n", pubKeyFile)
	fmt.Printf("Public key (for embedding in daemon):\n%s\n\n", pubKeyB64)
	fmt.Printf("To use license generation:\n")
	fmt.Printf("  dplane-license-gen --private-key %s --customer \"Name\" --audits 100 --ce-repo https://...\n", privKeyFile)
	fmt.Printf("\nTo use in daemon, set environment variable:\n")
	fmt.Printf("  export DPLANE_LICENSE_PUBLIC_KEY='%s'\n", pubKeyB64)
}
