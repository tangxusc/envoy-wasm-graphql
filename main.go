// Package main provides the entry point for the Envoy WASM GraphQL Federation Extension.
// This file is kept for compatibility but the actual WASM entry point is in cmd/wasm/main.go
package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("Envoy WASM GraphQL Federation Extension")
	fmt.Println("=======================================")
	fmt.Println("")
	fmt.Println("This is a GraphQL Federation extension for Envoy Proxy using WASM.")
	fmt.Println("")
	fmt.Println("To build the WASM extension:")
	fmt.Println("  make build")
	fmt.Println("")
	fmt.Println("To run the development environment:")
	fmt.Println("  docker-compose up -d")
	fmt.Println("")
	fmt.Println("To test the GraphQL federation:")
	fmt.Println("  curl -X POST http://localhost:8080/graphql \\")
	fmt.Println("    -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"query\": \"{ users { id name } }\"}'")
	fmt.Println("")
	fmt.Println("For more information, see README.md")

	log.Println("Note: The actual WASM entry point is in cmd/wasm/main.go")
}
