//go:build ignore

package main

import (
	"fmt"
	"os"

	"github.com/desek/outlook-local-mcp/internal/docs"
)

func main() {
	if err := os.WriteFile("llms.txt", []byte(docs.GenerateLLMsTxt()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
