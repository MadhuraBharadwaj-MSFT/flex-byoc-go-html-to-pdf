package main

import (
	"os"

	"github.com/MadhuraBharadwaj-MSFT/flex-byoc-go-html-to-pdf/internal/httpapi"
	"github.com/MadhuraBharadwaj-MSFT/flex-byoc-go-html-to-pdf/internal/pdf"
	"github.com/azure/azure-functions-golang-worker/sdk"
	"github.com/azure/azure-functions-golang-worker/worker"
)

func main() {
	renderer := pdf.NewChromiumRenderer(pdf.Options{
		ExecutablePath: os.Getenv("CHROMIUM_PATH"),
		MaxConcurrent:  1,
		MaxOutputBytes: 20 << 20,
		AllowNoSandbox: os.Getenv("CHROMIUM_NO_SANDBOX") == "1",
	})
	handler := httpapi.NewHandler(renderer, httpapi.Config{})

	app := sdk.FunctionApp()
	app.HTTP(
		"HtmlToPdf",
		handler.ServeHTTP,
		sdk.WithRoute("html-to-pdf"),
		sdk.WithMethods("GET", "POST"),
		sdk.WithAuth("anonymous"),
	)

	worker.Start(app)
}
