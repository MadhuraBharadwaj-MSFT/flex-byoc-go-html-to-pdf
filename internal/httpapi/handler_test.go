package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeRenderer struct {
	pdf       []byte
	err       error
	render    func(context.Context, string) ([]byte, error)
	lastHTML  string
	callCount int
}

func (f *fakeRenderer) Render(ctx context.Context, html string) ([]byte, error) {
	f.lastHTML = html
	f.callCount++
	if f.render != nil {
		return f.render(ctx, html)
	}
	return f.pdf, f.err
}

func TestHandlerGETRendersBuiltInInvoice(t *testing.T) {
	renderer := &fakeRenderer{pdf: []byte("%PDF-test")}
	handler := NewHandler(renderer, Config{})
	request := httptest.NewRequest(http.MethodGet, "/api/html-to-pdf", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if renderer.callCount != 1 {
		t.Fatalf("renderer calls = %d, want 1", renderer.callCount)
	}
	if !strings.Contains(renderer.lastHTML, "INVOICE") {
		t.Fatal("built-in invoice HTML was not rendered")
	}
	if got := response.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", got)
	}
	if got := response.Header().Get("Content-Disposition"); got != `attachment; filename="sample-invoice.pdf"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
	if !bytes.Equal(response.Body.Bytes(), renderer.pdf) {
		t.Fatal("response body does not match rendered PDF")
	}
}

func TestHandlerPOSTRendersHTML(t *testing.T) {
	renderer := &fakeRenderer{pdf: []byte("%PDF-post")}
	handler := NewHandler(renderer, Config{})
	request := httptest.NewRequest(http.MethodPost, "/api/html-to-pdf", strings.NewReader("<html><body>Custom</body></html>"))
	request.Header.Set("Content-Type", "text/html; charset=utf-8")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if renderer.lastHTML != "<html><body>Custom</body></html>" {
		t.Fatalf("renderer HTML = %q", renderer.lastHTML)
	}
	if got := response.Header().Get("Content-Disposition"); got != `attachment; filename="html-to-pdf.pdf"` {
		t.Fatalf("Content-Disposition = %q", got)
	}
}

func TestHandlerRejectsInvalidPOSTRequests(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		maxBytes    int64
		wantStatus  int
	}{
		{
			name:       "missing content type",
			body:       "<p>hello</p>",
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:        "wrong content type",
			contentType: "application/json",
			body:        "{}",
			wantStatus:  http.StatusUnsupportedMediaType,
		},
		{
			name:        "empty body",
			contentType: "text/html",
			body:        " \n\t ",
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "oversized body",
			contentType: "text/html",
			body:        "12345",
			maxBytes:    4,
			wantStatus:  http.StatusRequestEntityTooLarge,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			renderer := &fakeRenderer{pdf: []byte("%PDF-test")}
			handler := NewHandler(renderer, Config{MaxHTMLBytes: test.maxBytes})
			request := httptest.NewRequest(http.MethodPost, "/api/html-to-pdf", strings.NewReader(test.body))
			if test.contentType != "" {
				request.Header.Set("Content-Type", test.contentType)
			}
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if renderer.callCount != 0 {
				t.Fatalf("renderer calls = %d, want 0", renderer.callCount)
			}
		})
	}
}

func TestHandlerRejectsUnsupportedMethod(t *testing.T) {
	renderer := &fakeRenderer{pdf: []byte("%PDF-test")}
	handler := NewHandler(renderer, Config{})
	request := httptest.NewRequest(http.MethodPut, "/api/html-to-pdf", strings.NewReader("<p>hello</p>"))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusMethodNotAllowed)
	}
	if got := response.Header().Get("Allow"); got != "GET, POST" {
		t.Fatalf("Allow = %q, want GET, POST", got)
	}
}

func TestHandlerRejectsChunkedOversizedBody(t *testing.T) {
	renderer := &fakeRenderer{pdf: []byte("%PDF-test")}
	handler := NewHandler(renderer, Config{MaxHTMLBytes: 4})
	request := httptest.NewRequest(http.MethodPost, "/api/html-to-pdf", strings.NewReader("12345"))
	request.ContentLength = -1
	request.Header.Set("Content-Type", "text/html")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusRequestEntityTooLarge)
	}
	if renderer.callCount != 0 {
		t.Fatalf("renderer calls = %d, want 0", renderer.callCount)
	}
}

func TestHandlerReturnsRenderError(t *testing.T) {
	renderer := &fakeRenderer{err: errors.New("browser failed")}
	handler := NewHandler(renderer, Config{})
	request := httptest.NewRequest(http.MethodGet, "/api/html-to-pdf", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

func TestHandlerTimesOutRendering(t *testing.T) {
	renderer := &fakeRenderer{
		render: func(ctx context.Context, _ string) ([]byte, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	handler := NewHandler(renderer, Config{RenderTimeout: 5 * time.Millisecond})
	request := httptest.NewRequest(http.MethodGet, "/api/html-to-pdf", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusGatewayTimeout {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusGatewayTimeout)
	}
}
