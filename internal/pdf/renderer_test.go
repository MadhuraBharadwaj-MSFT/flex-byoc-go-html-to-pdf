package pdf

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestSecureHTMLBlocksActiveAndExternalContent(t *testing.T) {
	source := `<!doctype html>
<html>
<head>
  <link rel="stylesheet" href="https://example.com/site.css">
  <meta http-equiv="refresh" content="0;url=https://example.com">
  <style>body { color: #123; }</style>
</head>
<body onload="steal()">
  <script>steal()</script>
  <iframe src="https://example.com"></iframe>
  <img src="https://example.com/tracker.png" srcset="https://example.com/two.png 2x">
  <img src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw==">
  <a href="#section">Jump</a>
</body>
</html>`

	secured, err := secureHTML(source)
	if err != nil {
		t.Fatalf("secureHTML returned error: %v", err)
	}

	for _, forbidden := range []string{
		"<script",
		"<iframe",
		"<link",
		"http-equiv=\"refresh\"",
		"onload=",
		"https://example.com",
		"srcset=",
	} {
		if strings.Contains(strings.ToLower(secured), strings.ToLower(forbidden)) {
			t.Fatalf("secured HTML still contains %q", forbidden)
		}
	}
	if !strings.Contains(secured, "Content-Security-Policy") {
		t.Fatal("secured HTML is missing the Content Security Policy")
	}
	if !strings.Contains(secured, "data:image/gif") {
		t.Fatal("secured HTML removed an embedded data URL")
	}
	if !strings.Contains(secured, "href=\"#section\"") {
		t.Fatal("secured HTML removed an in-document anchor")
	}
}

func TestValidatePDF(t *testing.T) {
	if err := validatePDF([]byte("%PDF-valid"), 1024); err != nil {
		t.Fatalf("valid PDF returned error: %v", err)
	}
	if err := validatePDF([]byte("not-a-pdf"), 1024); err == nil {
		t.Fatal("invalid PDF did not return an error")
	}
	if err := validatePDF([]byte("%PDF-too-large"), 4); !errors.Is(err, ErrOutputTooLarge) {
		t.Fatalf("error = %v, want ErrOutputTooLarge", err)
	}
}

func TestChromiumRendererIntegration(t *testing.T) {
	if os.Getenv("HTML_TO_PDF_INTEGRATION") != "1" {
		t.Skip("set HTML_TO_PDF_INTEGRATION=1 to run with an installed Chromium browser")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	renderer := NewChromiumRenderer(Options{
		ExecutablePath: os.Getenv("CHROMIUM_PATH"),
		MaxConcurrent:  1,
		MaxOutputBytes: 5 << 20,
		AllowNoSandbox: os.Getenv("CHROMIUM_NO_SANDBOX") == "1",
	})
	pdfBytes, err := renderer.Render(ctx, "<html><body><h1>Integration test</h1></body></html>")
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	if len(pdfBytes) < 1000 {
		t.Fatalf("PDF length = %d, want at least 1000 bytes", len(pdfBytes))
	}
}
