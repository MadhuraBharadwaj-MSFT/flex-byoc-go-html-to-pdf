package pdf

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

var ErrOutputTooLarge = errors.New("rendered PDF exceeds the configured limit")

type Options struct {
	ExecutablePath string
	MaxConcurrent  int
	MaxOutputBytes int
	AllowNoSandbox bool
}

type ChromiumRenderer struct {
	executablePath string
	maxOutputBytes int
	allowNoSandbox bool
	slots          chan struct{}
}

func NewChromiumRenderer(options Options) *ChromiumRenderer {
	if options.MaxConcurrent <= 0 {
		options.MaxConcurrent = 1
	}
	if options.MaxOutputBytes <= 0 {
		options.MaxOutputBytes = 20 << 20
	}

	return &ChromiumRenderer{
		executablePath: options.ExecutablePath,
		maxOutputBytes: options.MaxOutputBytes,
		allowNoSandbox: options.AllowNoSandbox,
		slots:          make(chan struct{}, options.MaxConcurrent),
	}
}

func (r *ChromiumRenderer) Render(ctx context.Context, sourceHTML string) ([]byte, error) {
	securedHTML, err := secureHTML(sourceHTML)
	if err != nil {
		return nil, fmt.Errorf("secure HTML document: %w", err)
	}

	select {
	case r.slots <- struct{}{}:
		defer func() { <-r.slots }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	allocatorOptions := append(
		[]chromedp.ExecAllocatorOption(nil),
		chromedp.DefaultExecAllocatorOptions[:]...,
	)
	allocatorOptions = append(
		allocatorOptions,
		chromedp.DisableGPU,
		chromedp.Flag("proxy-server", "http://127.0.0.1:9"),
		chromedp.Flag("proxy-bypass-list", "<-loopback>"),
	)
	if r.executablePath != "" {
		allocatorOptions = append(allocatorOptions, chromedp.ExecPath(r.executablePath))
	}
	if r.allowNoSandbox {
		allocatorOptions = append(allocatorOptions, chromedp.NoSandbox)
	}

	allocatorContext, cancelAllocator := chromedp.NewExecAllocator(ctx, allocatorOptions...)
	defer cancelAllocator()

	browserContext, cancelBrowser := chromedp.NewContext(allocatorContext)
	defer cancelBrowser()

	if err := chromedp.Do(
		browserContext,
		chromedp.Navigate("about:blank"),
		chromedp.Func(func(ctx context.Context, _ *chromedp.Target) error {
			frameTree, err := chromedp.Call(ctx, page.GetFrameTree, cdp.Empty{})
			if err != nil {
				return err
			}
			_, err = chromedp.Call(ctx, page.SetDocumentContent, page.SetDocumentContentParams{
				FrameID: frameTree.FrameTree.Frame.ID,
				HTML:    securedHTML,
			})
			return err
		}),
		chromedp.WaitReady("body"),
	); err != nil {
		return nil, fmt.Errorf("load HTML in Chromium: %w", err)
	}

	if _, err := chromedp.Run(
		browserContext,
		chromedp.Evaluate[bool](`(async () => {
			if (document.fonts) {
				await document.fonts.ready;
			}
			await Promise.all(Array.from(document.images, (image) => {
				if (image.complete) {
					return Promise.resolve();
				}
				return new Promise((resolve) => {
					image.addEventListener("load", resolve, { once: true });
					image.addEventListener("error", resolve, { once: true });
				});
			}));
			return true;
		})()`, chromedp.EvalAwaitPromise),
	); err != nil {
		return nil, fmt.Errorf("wait for embedded document resources: %w", err)
	}

	pdfBytes, err := chromedp.Run(
		browserContext,
		chromedp.PrintToPDF(
			chromedp.PDFPaper(chromedp.PaperA4),
			chromedp.PDFMargins(0, 0, 0, 0),
			chromedp.PDFPreferCSSPageSize(),
			chromedp.PDFPrintBackground(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("print page to PDF: %w", err)
	}
	if err := validatePDF(pdfBytes, r.maxOutputBytes); err != nil {
		return nil, err
	}

	return pdfBytes, nil
}

func validatePDF(pdfBytes []byte, maxOutputBytes int) error {
	if len(pdfBytes) > maxOutputBytes {
		return ErrOutputTooLarge
	}
	if len(pdfBytes) < len("%PDF-") || !bytes.Equal(pdfBytes[:len("%PDF-")], []byte("%PDF-")) {
		return errors.New("Chromium returned an invalid PDF")
	}
	return nil
}
