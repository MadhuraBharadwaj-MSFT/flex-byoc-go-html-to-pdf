# Go Chromium HTML-to-PDF on Azure Functions Flex Consumption

This repository contains the Go worker-image scenario validated end to end on
Azure Functions Flex Consumption:

- `GET /api/hello` returns the frozen Gavin Go sample greeting.
- `GET /api/html-to-pdf` renders a built-in invoice.
- `POST /api/html-to-pdf` renders a bounded raw HTML document.

The image contains the compiled Go application/worker, OS-installed Chromium,
and fonts. The Functions Host and WorkerProxy remain Microsoft-managed outside
the image.

> [!IMPORTANT]
> Azure Functions Go worker-image support and the Go worker SDK used here are
> previews. Anonymous HTTP authorization is for demonstration only.

## Validated versions

| Component | Version |
|---|---|
| Go | `1.26.0` module / Go 1.26 Bookworm builder |
| Azure Functions Go SDK/worker | `v0.7.0-preview` |
| chromedp | `v0.23.0` |
| Chromium | Debian Bookworm `chromium-headless-shell` |
| Container target | Linux AMD64 |
| Runtime user | UID/GID `10001:10001` |

Both base images are pinned to the digests used during validation.

## Runtime contract

The image runs the compiled worker directly:

```text
/home/site/wwwroot/app
```

The Flex platform supplies:

```text
--functions-uri
--functions-worker-id
--functions-request-id
--functions-grpc-max-message-length
```

No startup adapter, Functions Host, WorkerProxy, Core Tools, placeholder proxy,
Go compiler, Python, or Node.js runtime is included in the final image.

The image sets only the Go worker transport requirement:

```text
FUNCTIONS_GO_DISABLE_HTTP_PROXY=1
```

It does not require `AzureWebJobsScriptRoot`,
`FUNCTIONS_APPLICATION_DIRECTORY`, `FUNCTIONS_WORKER_RUNTIME`, or
`FUNCTIONS_WORKER_RUNTIME_VERSION`.

## Repository layout

```text
.
|-- examples/custom.html          # Self-contained POST example
|-- main.go                       # Function registration and worker startup
|-- handler.go                    # Request validation and response handling
|-- renderer.go                   # Sandboxed Chromium lifecycle and PDF output
|-- main_test.go                  # Handler and compatibility tests
|-- renderer_integration_test.go  # Real Chromium and network-blocking test
|-- Dockerfile                    # Integration and final image targets
|-- host.json
|-- go.mod / go.sum
```

## Behavior and bounds

| Control | Value |
|---|---|
| Accepted methods | GET and POST |
| POST body | Raw HTML |
| Maximum HTML | 256 KiB |
| Render timeout | 20 seconds |
| Maximum PDF | 8 MiB |
| Maximum printed pages | 20 |
| Concurrent renders per worker | 1 |
| JavaScript | Disabled |
| External HTTP/HTTPS/file/FTP/WS/WSS resources | Blocked |
| Chromium sandbox | Enabled |
| Container user | Non-root |

There is no endpoint that accepts or fetches a caller-provided URL.

## Test the Go source

With Go 1.26 installed:

```bash
go test -count=1 ./...
go vet ./...
```

Or run the same checks in the builder image:

```bash
docker run --rm --platform linux/amd64 \
  --mount "type=bind,source=$PWD,target=/src" \
  --workdir /src \
  golang:1.26-bookworm \
  sh -c 'go test -count=1 ./... && go vet ./...'
```

## Run the real Chromium integration test

Build the integration target:

```bash
docker build \
  --platform linux/amd64 \
  --target integration \
  --tag flex-byoc-go-html-to-pdf:integration \
  .
```

Docker Desktop's default seccomp profile can block Chromium namespace creation.
The following test-only allowance keeps Chromium's own sandbox enabled; the
image contains no `--no-sandbox` fallback:

```bash
docker run --rm \
  --platform linux/amd64 \
  --security-opt seccomp=unconfined \
  --env RUN_CHROMIUM_INTEGRATION=1 \
  --env PDF_OUTPUT_PATH=/evidence/integration.pdf \
  --mount "type=bind,source=$PWD,target=/evidence" \
  --entrypoint /bin/sh \
  flex-byoc-go-html-to-pdf:integration \
  -c '/tmp/pdf.test -test.v -test.run TestChromiumRendererIntegration'
```

The integration test verifies:

- the process runs as UID `10001`;
- no Chromium sandbox-disabling argument is present;
- external page resources make zero requests;
- the output starts with `%PDF-` and contains `%%EOF`.

## Build the final worker image

```bash
docker build \
  --platform linux/amd64 \
  --target final \
  --tag flex-byoc-go-html-to-pdf:latest \
  .
```

This is a worker-only image. It does not expose a standalone HTTP port, so
`docker run -p ...` is not an endpoint test.

## Deploy to Flex Consumption

The validated flow is:

1. Create an ordinary Linux Flex Consumption Go `1.0` app with valid Functions
   storage and Application Insights.
2. Push this Linux AMD64 image to a private registry using a unique tag.
3. Resolve the registry manifest/index digest.
4. Pin the deployment as `registry/repository:tag@sha256:...`.
5. Preserve the existing `functionAppConfig`, clear `runtime`, and change only
   `deployment.storage` to `Registry`.
6. Configure a repository-scoped read credential or an approved managed
   identity pull path.
7. Verify the public Functions endpoint. A control-plane `Running` state is not
   a functional pass.

For the current Go worker mechanics and Stage prototype deployment shape, see
[Gavin Aguiar's Go BYOC guide](https://gist.github.com/gavin-aguiar/1e7d6dcd7cf4c7d1b8a3c5e255506eb8).

Use immutable image digests. Do not rely on a mutable tag and do not add a
Functions Host or a direct HTTP listener to the image.

## Invoke the deployed app

Set the endpoint:

```bash
ENDPOINT="https://<function-app-hostname>/api"
```

Verify the original Go sample:

```bash
curl --fail --show-error "$ENDPOINT/hello"
```

Expected body:

```text
Hello from the BYOC Go worker.
```

Download the built-in invoice:

```bash
curl --fail --show-error \
  "$ENDPOINT/html-to-pdf" \
  --output invoice-get.pdf
```

Render the included custom HTML:

```bash
curl --fail --show-error \
  --request POST \
  --header "Content-Type: text/html; charset=utf-8" \
  --data-binary @examples/custom.html \
  "$ENDPOINT/html-to-pdf" \
  --output invoice-post.pdf
```

Verify the signatures:

```bash
head -c 5 invoice-get.pdf
head -c 5 invoice-post.pdf
```

Both commands must print:

```text
%PDF-
```

Open the files in a browser or PDF viewer for the visual check.

## Preview worker Content-Type compatibility

In the validated Stage flow, externally sent `Content-Type` was delivered in
`RpcHttp.NullableHeaders`, while Go worker `v0.7.0-preview` copies only
`RpcHttp.Headers` into `http.Request`.

The handler therefore:

- accepts a missing media type for POST on this preview worker path;
- still rejects any visible non-`text/html` media type;
- always applies the same input, render, output, network, and concurrency
  limits.

The client should still send `Content-Type: text/html`.

## Security notes

Rendering untrusted HTML invokes a browser parser and remains high risk.

This sample:

- runs the app and Chromium as non-root;
- installs and retains Chromium's sandbox;
- disables JavaScript;
- blocks external subresources;
- limits input, pages, output, duration, and concurrency;
- uses a new temporary browser profile for each request and removes it;
- returns `Cache-Control: no-store` and `X-Content-Type-Options: nosniff`;
- contains no registry or Azure credentials.

For production, add platform authentication, authorization, rate limiting,
tenant isolation, abuse monitoring, and workload-specific HTML sanitization.
Rebuild frequently for Chromium and OS security updates.

## License

This sample is licensed under the [MIT License](LICENSE).
