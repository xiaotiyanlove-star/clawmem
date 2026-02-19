package main

import (
	"context"
	"fmt"
	"os"

	"github.com/philippgille/chromem-go"
)

func main() {
	fmt.Println("Testing chromem default...")
	f := chromem.NewEmbeddingFuncDefault()
	_, err := f(context.Background(), "test")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Success")
}
