package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/ledongthuc/pdf"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run debug_pdf.go <pdf-file>")
		os.Exit(1)
	}

	path := os.Args[1]
	f, r, err := pdf.Open(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	totalPage := r.NumPage()
	fmt.Printf("Total pages: %d\n\n", totalPage)

	for i := 1; i <= totalPage; i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}

		texts := p.Content().Text
		fmt.Printf("=== PAGE %d ===\n", i)

		for idx, t := range texts {
			fmt.Printf("[%d] X:%.2f Y:%.2f Font:%s Text: %q\n", idx, t.X, t.Y, t.Font, strings.TrimSpace(t.S))
		}
		fmt.Println()
	}
}
