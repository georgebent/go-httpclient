# Go HTTP Client

A standard-library-first HTTP client for Go with a builder API, request options, context support, bounded response bodies, response metadata, and test-friendly mocking.

## Installation

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
	SetMaxIdleConnections(10).
	SetMaxBodyBytes(1024 * 1024).
	SetUserAgent("your-user-agent").
	Build()
```

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

If you provide `SetHttpClient(...)`, it has the highest priority. Builder transport and redirect settings are not overlaid onto that client.

## Error helpers

Helpers are available for common network failures:

```go
if gohttp.IsTimeout(err) { ... }
if gohttp.IsDNS(err) { ... }
if gohttp.IsTLS(err) { ... }
if gohttp.IsConnection(err) { ... }
if gohttp.IsCanceled(err) { ... }

kind := gohttp.ClassifyError(err)
```

## Testing

The recommended path is transport-level mocking.

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

Legacy global mocking is still available for backward compatibility:

```go
gohttp_mock.StartMockServer()
defer gohttp_mock.StopMockServer()
gohttp_mock.MockupServer.DeleteMocks()

gohttp_mock.AddMock(gohttp_mock.Mock{
	Method:         http.MethodGet,
	Url:            "http://localhost",
	ResponseStatus: http.StatusOK,
	ResponseBody:   "ok",
})
```

## Examples

See `examples/get.go`, `examples/post.go`, and `examples/singleton_client.go`.

## Linting

```bash
golangci-lint run ./...
```

## Collaboration

Pull requests are welcome. For major changes, open an issue first.
