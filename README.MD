# Go HTTP Client

A standard-library-first HTTP client for Go with a builder API, request options, context support, bounded response bodies, response metadata, and test-friendly mocking.

## Installation

Requires Go 1.26 or newer.

```bash
go get github.com/georgebent/go-httpclient
```

## Usage

Import the client package:

```go
import "github.com/georgebent/go-httpclient/gohttp"
```

## Build a client

```go
headers := make(http.Header)
headers.Set("Content-Type", "application/json")
headers.Set("X-Common-Header", "value-for-all-requests")

client := gohttp.NewBuilder().
	SetHeaders(headers).
	SetConnectionTimeout(2 * time.Second).
	SetResponseTimeout(3 * time.Second).
	SetRequestTimeout(30 * time.Second).
	SetMaxIdleConnections(10).
	SetMaxBodyBytes(1024 * 1024).
	SetUserAgent("your-user-agent").
	Build()
```

Timeouts:

- `SetConnectionTimeout` limits dialing a connection (default `1s`).
- `SetResponseTimeout` limits waiting for response headers after the request is sent (default `5s`). It does not limit reading the body.
- `SetRequestTimeout` limits the whole request, including redirects and reading the body (default `30s`). Raise it or use a `context` deadline for large downloads.
- `DisableTimeouts(true)` turns off all default timeouts. Timeouts set explicitly still apply.

Connection and response timeouts apply to the built-in transport only. With `SetTransport(...)`, only the request timeout and the request context apply.

The built-in transport is a clone of `http.DefaultTransport`, so proxy settings from the environment, HTTP/2, and TLS handshake timeouts work as in the standard library.

Builder headers are sent with every request. A request header with the same name replaces the builder header; all values of repeated headers are kept.

## Perform requests

The new core API is `Do(ctx, method, url, opts...)`.

```go
response, err := client.Do(
	context.Background(),
	http.MethodPost,
	"https://api.example.com/users",
	gohttp.WithHeaders(http.Header{
		"Authorization": []string{"Bearer ABC-123"},
	}),
	gohttp.WithBody(map[string]string{
		"firstname": "John",
		"lastname":  "Stranger",
	}),
	gohttp.WithMaxBodyBytes(64*1024),
)
if err != nil {
	return err
}

fmt.Println(response.StatusCode)
fmt.Println(response.BodyString())
fmt.Println(response.FinalURL)
fmt.Println(response.ContentLength)
fmt.Println(response.Duration)
fmt.Println(response.RedirectCount)
```

Convenience methods are also available:

```go
response, err := client.Get(context.Background(), "https://example.com/health")
```

## Request options

Available request options:

- `gohttp.WithHeaders(headers)`
- `gohttp.WithBody(body)`
- `gohttp.WithBodyReader(reader)`
- `gohttp.WithMaxBodyBytes(limit)`

`WithBody(...)` and `WithBodyReader(...)` are mutually exclusive.

`WithBody(...)` encodes the body as follows:

- `[]byte`, `string`, and any `io.Reader` are sent as is.
- `url.Values` is form-encoded when `Content-Type` is `application/x-www-form-urlencoded` or not set; in the latter case that `Content-Type` is added.
- Other values are encoded by the media type of `Content-Type` (parameters such as `charset` are ignored):
  - JSON for `application/json` and `*+json`;
  - XML for `application/xml`, `text/xml`, and `*+xml`;
  - JSON with an added `Content-Type: application/json` when no `Content-Type` is set.
- Any other combination fails with `gohttp.ErrUnsupportedContentType`, for example a struct with `text/plain`. Send such bodies as `[]byte`, `string`, or a reader.

Response bodies are limited to 10 MiB by default. Set another limit with `SetMaxBodyBytes(...)` on the builder or `WithMaxBodyBytes(...)` per request; a negative value removes the limit.

If the response body exceeds the limit, the client returns `response, gohttp.ErrBodyTooLarge`.
The response metadata remains populated, and `response.Body` contains the first `limit` bytes.

## Response model

The response contains both payload and metadata:

```go
type Response struct {
	Status        string
	StatusCode    int
	Headers       http.Header
	Body          []byte
	FinalURL      string
	ContentLength int64
	Duration      time.Duration
	RedirectCount int
}
```

## Custom transport and redirects

You can inject a custom `*http.Client`, a custom `http.RoundTripper`, or redirect handling:

```go
client := gohttp.NewBuilder().
	SetTransport(customTransport).
	SetRedirectPolicy(gohttp.RedirectPolicy{
		MaxRedirects: 3,
		Validate: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}).
	Build()
```

Redirect limits:

- `MaxRedirects: 0` (or no policy) follows up to 10 redirects.
- `MaxRedirects: n` follows up to `n` redirects.
- `MaxRedirects: -1` does not follow redirects; the first `3xx` response is returned without an error.

When the limit is exceeded, the request fails with an error that matches `gohttp.ErrTooManyRedirects` and `gohttp.IsRedirectError`.

If you provide `SetHttpClient(...)`, it has the highest priority. Builder transport and redirect settings are not overlaid onto that client.

## Error helpers

Helpers are available for common network failures:

```go
if gohttp.IsTimeout(err) { ... }
if gohttp.IsDNS(err) { ... }
if gohttp.IsTLS(err) { ... }
if gohttp.IsConnection(err) { ... }
if gohttp.IsCanceled(err) { ... }
if gohttp.IsRedirectError(err) { ... }

kind := gohttp.ClassifyError(err)
```

`IsTimeout` covers network timeouts, `SetRequestTimeout`, and expired `context` deadlines. `IsCanceled` covers only `context` cancellation. `IsTLS` covers record header errors, TLS alerts sent or received during the handshake, and certificate verification errors.

## Testing

Mocks are set per client through `SetTransport(...)`, so the request goes through the same redirect and timeout handling as in production.

```go
mockTransport := gohttp_mock.NewTransport()
mockTransport.AddMock(gohttp_mock.Mock{
	Method:         http.MethodGet,
	Url:            "https://api.example.com/health",
	ResponseStatus: http.StatusOK,
	ResponseHeaders: http.Header{
		"X-Mock": []string{"true"},
	},
	ResponseBody: "ok",
})

client := gohttp.NewBuilder().
	SetTransport(mockTransport).
	Build()

response, err := client.Get(context.Background(), "https://api.example.com/health")
```

The global mock server (`StartMockServer`, `StopMockServer`, `MockupServer`, `gohttp_mock.AddMock`) was removed. To mock a shared client, build it from a function that accepts a transport, as in `examples/singleton_client.go` and `examples/get_test.go`.

## Examples

See `examples/get.go`, `examples/post.go`, and `examples/singleton_client.go`.

## Development

Tests run with the race detector in CI:

```bash
go test -race ./...
```

Linting uses golangci-lint v2 (CI pins v2.15.0) with the config in `.golangci.yml`:

```bash
golangci-lint run ./...
```

Apply modernizations suggested by the Go toolchain:

```bash
go fix ./...
```

## Collaboration

Pull requests are welcome. For major changes, open an issue first.
