package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/MadhuraBharadwaj-MSFT/flex-byoc-go-html-to-pdf/internal/sample"
)

const (
	defaultMaxHTMLBytes  = int64(1 << 20)
	defaultRenderTimeout = 30 * time.Second
)

type Renderer interface {
	Render(context.Context, string) ([]byte, error)
}

type Config struct {
	MaxHTMLBytes  int64
	RenderTimeout time.Duration
}

type Handler struct {
	renderer      Renderer
	maxHTMLBytes  int64
	renderTimeout time.Duration
}

func NewHandler(renderer Renderer, config Config) *Handler {
	if config.MaxHTMLBytes <= 0 {
		config.MaxHTMLBytes = defaultMaxHTMLBytes
	}
	if config.RenderTimeout <= 0 {
		config.RenderTimeout = defaultRenderTimeout
	}

	return &Handler{
		renderer:      renderer,
		maxHTMLBytes:  config.MaxHTMLBytes,
		renderTimeout: config.RenderTimeout,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var (
		sourceHTML string
		filename   string
	)

	switch r.Method {
	case http.MethodGet:
		sourceHTML = sample.InvoiceHTML
		filename = "sample-invoice.pdf"
	case http.MethodPost:
		var ok bool
		sourceHTML, ok = h.readHTMLBody(w, r)
		if !ok {
			return
		}
		filename = "html-to-pdf.pdf"
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method must be GET or POST")
		return
	}

	renderContext, cancel := context.WithTimeout(r.Context(), h.renderTimeout)
	defer cancel()

	pdfBytes, err := h.renderer.Render(renderContext, sourceHTML)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to render HTML as PDF", "error", err)
		if errors.Is(err, context.DeadlineExceeded) {
			writeError(w, http.StatusGatewayTimeout, "PDF rendering timed out")
			return
		}
		if errors.Is(err, context.Canceled) {
			writeError(w, http.StatusRequestTimeout, "PDF rendering was canceled")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to render PDF")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Content-Length", strconv.Itoa(len(pdfBytes)))
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(pdfBytes); err != nil {
		slog.ErrorContext(r.Context(), "failed to write PDF response", "error", err)
	}
}

func (h *Handler) readHTMLBody(w http.ResponseWriter, r *http.Request) (string, bool) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.EqualFold(mediaType, "text/html") {
		writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be text/html")
		return "", false
	}

	if r.ContentLength > h.maxHTMLBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "HTML body exceeds the 1 MiB limit")
		return "", false
	}

	body := http.MaxBytesReader(w, r.Body, h.maxHTMLBytes)
	defer body.Close()

	htmlBytes, err := io.ReadAll(body)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			writeError(w, http.StatusRequestEntityTooLarge, "HTML body exceeds the 1 MiB limit")
			return "", false
		}
		writeError(w, http.StatusBadRequest, "failed to read HTML body")
		return "", false
	}

	sourceHTML := string(htmlBytes)
	if strings.TrimSpace(sourceHTML) == "" {
		writeError(w, http.StatusBadRequest, "HTML body must not be empty")
		return "", false
	}

	return sourceHTML, true
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}
