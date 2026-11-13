<!--
---
name: Go HTML-to-PDF on Azure Functions Flex Consumption containers
description: >-
  This sample deploys a Go 1.27 Azure Functions worker container that converts
  bounded HTML to PDF with chromedp and operating-system Chromium. Azure
  Developer CLI provisions the supporting resources and deploys an immutable
  container image.
page_type: sample
languages:
- go
- bicep
- azdeveloper
- dockerfile
products:
- azure
- azure-functions
- azure-container-registry
urlFragment: flex-container-go-html-to-pdf
---
-->

# Go HTML-to-PDF on Azure Functions Flex Consumption containers

This sample accepts self-contained HTML and returns a PDF rendered with
[chromedp](https://github.com/chromedp/chromedp) and the operating system's
Chromium package.

The Azure Functions app runs from a Go 1.27 Flex Consumption container. Azure
Developer CLI (`azd`) provisions the Azure resources, builds the Linux AMD64
container with Azure Container Registry Tasks, deploys it by immutable digest,
and verifies the live endpoints.

![Architecture diagram for Go HTML-to-PDF on Azure Functions Flex Consumption containers](./img/architecture.svg)

> [!IMPORTANT]
> Azure Functions Flex Consumption container support is currently in preview.
> Preview capabilities and deployment requirements may change before general
> availability.

## Sample workflow

1. Clone the repository.
1. Run `azd up` to provision, build, deploy, and verify the Go container.
1. Call `GET /api/html-to-pdf` for the built-in invoice or POST your own HTML.
1. Save or display the returned `application/pdf` response.

The final image contains only the customer-owned Go worker and its operating
system dependencies. Azure Functions provides the managed hosting layer.

## Prerequisites

To deploy the sample:

- [Git](https://git-scm.com/downloads)
- [Python 3](https://www.python.org/downloads/) for deterministic image
  packaging and cross-platform verification scripts
- [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli)
- [Azure Developer CLI](https://learn.microsoft.com/azure/developer/azure-developer-cli/install-azd)
- an Azure subscription enabled for the Registry-backed Flex Consumption
  container preview
- permission to create resource groups, role assignments, storage, networking,
  monitoring, Azure Container Registry, FC1 plans, and Function Apps

Optional local validation also uses:

- [Go 1.27.0](https://go.dev/dl/)
- [Docker Desktop](https://docs.docker.com/desktop/) with Linux AMD64 support

## Initialize the project

```shell
git clone https://github.com/MadhuraBharadwaj-MSFT/flex-container-go-html-to-pdf.git
cd flex-container-go-html-to-pdf
```

## How it works

[`main.go`](./main.go) registers two anonymous HTTP functions:

```text
GET|POST /api/hello
GET|POST /api/html-to-pdf
```

`/api/hello` returns the exact baseline response:

```text
Hello from the BYOC Go worker.
```

[`handler.go`](./handler.go) implements the HTML-to-PDF contract:

- GET renders a built-in invoice.
- POST accepts bounded raw HTML.
- Visible non-`text/html` media types are rejected.
- HTML input is limited to 256 KiB.
- PDF responses use `application/pdf` and `Cache-Control: no-store`.

[`renderer.go`](./renderer.go):

1. allows one browser render at a time;
1. creates a temporary Chromium profile;
1. starts Chromium as the non-root app user;
1. disables JavaScript and unnecessary browser services;
1. blocks HTTP, HTTPS, file, FTP, and WebSocket subresources;
1. loads HTML through a local `data:` URL;
1. prints at most 20 pages;
1. enforces a 20-second timeout and 8 MiB output limit;
1. validates the PDF signature;
1. cancels Chromium and removes the temporary profile.

Exact runtime and dependency pins live in the
[`Dockerfile`](./Dockerfile) and [`go.mod`](./go.mod).

## How the Azure environment is secured

The Bicep deployment:

- creates a dedicated user-assigned managed identity;
- disables shared-key and public network access on runtime storage;
- integrates the Function App with a dedicated Flex Consumption subnet;
- reaches Blob Storage through a private endpoint and private DNS;
- uses managed identity for runtime storage and Application Insights;
- keeps ACR admin and anonymous pull disabled;
- gives the Function App a repository-scoped read-only ACR token;
- disables FTP, SCM basic authentication, and remote debugging;
- enforces HTTPS and TLS 1.2 or newer.

The Function App endpoint remains public and anonymous to keep the sample easy
to test. Add platform authentication, authorization, rate limiting, tenant
isolation, and abuse monitoring before production use.

## How the container image is built

The deployment uses
[Azure Container Registry Tasks](https://learn.microsoft.com/azure/container-registry/container-registry-tasks-overview)
to build the Linux AMD64 image in Azure. Customers don't need local Docker to
deploy the sample.

During `azd up`:

1. [`imagecontext.py`](./infra/scripts/imagecontext.py) stages only
   `.dockerignore`, `Dockerfile`, `go.mod`, `go.sum`, `host.json`, and
   root-level Go source files.
1. The staged context receives a deterministic SHA-256 hash.
1. The hash becomes the image tag: `azd-<context-hash>`.
1. An existing tag is reused; otherwise `az acr build` runs an ACR Tasks quick
   build.
1. The workflow resolves the registry manifest digest.
1. The Function App is pinned to
   `registry/repository:tag@sha256:digest`.
1. The tag is write- and delete-locked so its digest can't move.

## Deploy to Azure

Sign in:

```shell
az login
azd auth login
```

Create an `azd` environment and set the required deployment locations:

```shell
azd env new go-pdf-flex --no-prompt
azd env set AZURE_SUBSCRIPTION_ID <subscription-id>
azd env set AZURE_LOCATION eastus2
azd env set AZURE_FUNCTION_LOCATION eastus2euap
```

`AZURE_LOCATION` is used for storage, ACR, Log Analytics, and Application
Insights. `AZURE_FUNCTION_LOCATION` is used for the FC1 plan, Function App,
virtual network, and private endpoint. The split is required because the
preview Function location doesn't support every monitoring resource.

Provision the resources, build the container, deploy it, and verify all HTTP
paths:

```shell
azd up
```

For unattended execution, use `azd up --no-prompt` after all four environment
values are set.

The postdeploy verification checks:

- Hello GET and POST return the exact expected body;
- PDF GET and `text/html` POST return complete PDFs;
- `application/json` POST returns HTTP 415;
- Azure inventories exactly `Hello` and `HtmlToPdf`;
- the app uses the expected immutable Registry image, managed identity, and
  VNet subnet;
- incompatible deployment settings are absent.

## Test the deployed container

Get the deployed endpoint.

PowerShell:

```powershell
$apiUrl = azd env get-value API_URL
```

Bash:

```bash
API_URL=$(azd env get-value API_URL)
```

Test the baseline endpoint:

```shell
curl --fail --show-error "$API_URL/api/hello"
```

Expected response:

```text
Hello from the BYOC Go worker.
```

Get the built-in invoice:

PowerShell:

```powershell
$format = "HTTP %{http_code}; Content-Type %{content_type}; " +
  "Bytes %{size_download}`n"
curl.exe --fail --show-error `
  --output invoice.pdf `
  --write-out $format `
  "$apiUrl/api/html-to-pdf"
```

Bash:

```bash
FORMAT='HTTP %{http_code}; Content-Type %{content_type}; Bytes %{size_download}\n'
curl --fail --show-error \
  --output invoice.pdf \
  --write-out "$FORMAT" \
  "$API_URL/api/html-to-pdf"
```

POST the included [`examples/custom.html`](./examples/custom.html).

PowerShell:

```powershell
$format = "HTTP %{http_code}; Content-Type %{content_type}; " +
  "Bytes %{size_download}`n"
curl.exe --fail --show-error `
  --request POST `
  --header "Content-Type: text/html; charset=utf-8" `
  --data-binary "@examples/custom.html" `
  --output custom.pdf `
  --write-out $format `
  "$apiUrl/api/html-to-pdf"
```

Bash:

```bash
FORMAT='HTTP %{http_code}; Content-Type %{content_type}; Bytes %{size_download}\n'
curl --fail --show-error \
  --request POST \
  --header "Content-Type: text/html; charset=utf-8" \
  --data-binary @examples/custom.html \
  --output custom.pdf \
  --write-out "$FORMAT" \
  "$API_URL/api/html-to-pdf"
```

Both PDFs must be non-empty and contain valid PDF boundaries:

```shell
python -c "from pathlib import Path
for name in ('invoice.pdf', 'custom.pdf'):
    data = Path(name).read_bytes()
    assert data.startswith(b'%PDF-')
    assert data.rstrip().endswith(b'%%EOF')
print('Valid PDFs: invoice.pdf, custom.pdf')"
```

Verify that a visible non-HTML media type is rejected:

```shell
curl --silent --show-error \
  --request POST \
  --header "Content-Type: application/json" \
  --data-binary '{}' \
  --output invalid-response.txt \
  --write-out "HTTP %{http_code}\n" \
  "$API_URL/api/html-to-pdf"
```

Expected status:

```text
HTTP 415
```

## Run focused checks locally

With Go 1.27.0 installed:

```shell
go test -count=1 ./...
go vet ./...
```

Or use the pinned builder family:

```shell
docker run --rm --platform linux/amd64 \
  --mount "type=bind,source=$PWD,target=/src" \
  --workdir /src \
  golang:1.27.0-bookworm \
  sh -c 'go test -count=1 ./... && go vet ./...'
```

Build and run the real Chromium integration test:

```shell
docker build \
  --platform linux/amd64 \
  --target integration \
  --tag flex-container-go-html-to-pdf:integration \
  .

mkdir -p evidence
docker run --rm \
  --security-opt seccomp=unconfined \
  --env RUN_CHROMIUM_INTEGRATION=1 \
  --env PDF_OUTPUT_PATH=/evidence/integration.pdf \
  --mount "type=bind,source=$PWD/evidence,target=/evidence" \
  flex-container-go-html-to-pdf:integration
```

Docker Desktop's default seccomp profile can block Chromium namespace creation.
The local test relaxes Docker's seccomp profile only; Chromium's own sandbox
remains enabled. Azure runs the normal image without this test-only option.

Build the deployable worker-only image:

```shell
docker build \
  --platform linux/amd64 \
  --target final \
  --tag flex-container-go-html-to-pdf:latest \
  .
```

The worker-only image requires the Azure Functions host and isn't intended to
be called directly with local `curl`.

## Redeploy container changes

Rerun `azd up`, or use the image-only workflow after the environment is
provisioned:

PowerShell:

```powershell
.\infra\scripts\deployimage.ps1 -Verify
```

Bash:

```bash
sh ./infra/scripts/deployimage.sh --verify
```

Unchanged image inputs reuse the existing digest. The image deployment
generates a repository-scoped ACR password with a 30-day expiration. Rerun the
deployment before that credential expires so future cold starts can continue
to pull the image.

## Content-Type compatibility

The sample uses the Go worker's default HTTP request path, which preserves
`Content-Type` in the validated Azure configuration.

The handler retains support for a truly missing POST media type in case the
platform falls back to HTTP over gRPC. That fallback is affected by
[Azure/azure-functions-golang-worker#87](https://github.com/Azure/azure-functions-golang-worker/issues/87).
Clients should always send `Content-Type: text/html`.

## Preview limitations

- The direct Registry deployment uses preview Azure resource APIs.
- Region and subscription enablement are required.
- ACR Basic uses a public service endpoint; admin and anonymous access remain
  disabled.
- Runtime storage is private, but the HTTP Function endpoint is public.
- The HTTP functions are anonymous.
- The image is Linux AMD64 only.
- The short-lived ACR pull password must be refreshed by rerunning deployment.

For the advanced manual deployment shape, see
[`docs/deploy-stage.md`](./docs/deploy-stage.md).

## Costs

The environment can incur charges for:

- Flex Consumption executions;
- Azure Container Registry storage and ACR Tasks builds;
- the storage account and private endpoint;
- Log Analytics and Application Insights ingestion;
- Chromium CPU time and cold starts.

## Clean up resources

Review the selected environment:

```shell
azd env get-values
```

Delete the resource group and all resources created for that environment:

```shell
azd down --force
```

This command doesn't delete the local repository.

## Resources

- [Azure Developer CLI](https://learn.microsoft.com/azure/developer/azure-developer-cli/)
- [Azure Functions Go developer guide](https://learn.microsoft.com/azure/azure-functions/functions-reference-go)
- [Azure Functions Flex Consumption](https://learn.microsoft.com/azure/azure-functions/flex-consumption-plan)
- [Azure Functions networking options](https://learn.microsoft.com/azure/azure-functions/functions-networking-options)
- [Azure Container Registry Tasks](https://learn.microsoft.com/azure/container-registry/container-registry-tasks-overview)
- [chromedp](https://github.com/chromedp/chromedp)

## License

This sample is licensed under the [MIT License](LICENSE).
