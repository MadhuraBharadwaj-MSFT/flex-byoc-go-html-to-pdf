package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MadhuraBharadwaj-MSFT/flex-byoc-go-html-to-pdf/internal/pdf"
	"github.com/MadhuraBharadwaj-MSFT/flex-byoc-go-html-to-pdf/internal/sample"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	renderer := pdf.NewChromiumRenderer(pdf.Options{
		ExecutablePath: os.Getenv("CHROMIUM_PATH"),
		MaxConcurrent:  1,
		MaxOutputBytes: 20 << 20,
		AllowNoSandbox: os.Getenv("CHROMIUM_NO_SANDBOX") == "1",
	})
	pdfBytes, err := renderer.Render(ctx, sample.InvoiceHTML)
	if err != nil {
		fmt.Fprintf(os.Stderr, "render check failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("render check passed: generated %d PDF bytes\n", len(pdfBytes))
}
