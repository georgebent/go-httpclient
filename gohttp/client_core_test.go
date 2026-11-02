package gohttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/georgebent/go-httpclient/gohttp_mock"
	"github.com/georgebent/go-httpclient/gomime"
)

type roundTripperFunc func(request *http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestGetRequestHeaders(t *testing.T) {

	commonHeaders := make(http.Header)
	commonHeaders.Set("Content-Type", "application/json")
	commonHeaders.Set("User-Agent", "cool-http-client")

	builder := clientBuilder{
		headers: commonHeaders,
	}

	client := Client{
		builder: &builder,
	}

	requestHeaders := make(http.Header)
	requestHeaders.Set("X-Request-Id", "123")

	finalHeaders := client.getRequestHeaders(requestHeaders)

	if len(finalHeaders) != 3 {
		t.Error("We expect 3 headers")
	}

	if finalHeaders.Get("Content-Type") != "application/json" {
		t.Error("Invalid Content-Type received")
	}

	if finalHeaders.Get("User-Agent") != "cool-http-client" {
		t.Error("Invalid User-Agent received")
	}

	if finalHeaders.Get("X-Request-Id") != "123" {
		t.Error("Invalid X-Request-Id received")
	}
}

func TestGetRequestHeadersKeepsAllValues(t *testing.T) {
	commonHeaders := http.Header{}
	commonHeaders.Add("Accept", "application/json")
	commonHeaders.Add("Accept", "text/plain")
	commonHeaders.Add("X-Common", "one")

	client := Client{builder: &clientBuilder{headers: commonHeaders}}

	requestHeaders := http.Header{}
	requestHeaders.Add("X-Common", "two")
	requestHeaders.Add("X-Common", "three")
	requestHeaders["x-lower"] = []string{"value"}

	finalHeaders := client.getRequestHeaders(requestHeaders)

	if got := finalHeaders.Values("Accept"); len(got) != 2 || got[0] != "application/json" || got[1] != "text/plain" {
		t.Fatalf("expected both Accept values from builder, got %v", got)
	}

	if got := finalHeaders.Values("X-Common"); len(got) != 2 || got[0] != "two" || got[1] != "three" {
		t.Fatalf("expected request values to replace builder values, got %v", got)
	}

	if finalHeaders.Get("X-Lower") != "value" {
		t.Fatalf("expected non-canonical header key to be canonicalized, got %v", finalHeaders)
	}

	requestHeaders.Set("X-Common", "changed")
	if finalHeaders.Get("X-Common") != "two" {
		t.Fatal("expected request headers to be copied, not shared")
	}
}

func TestGetRequestBody(t *testing.T) {
	client := Client{}

	readBody := func(t *testing.T, body io.Reader) string {
		t.Helper()

		payload, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("expected readable body, got %v", err)
		}

		return string(payload)
	}

	headersWith := func(contentType string) http.Header {
		headers := make(http.Header)
		if contentType != "" {
			headers.Set(gomime.HEADER_CONTENT_TYPE, contentType)
		}

		return headers
	}

	t.Run("noBodyNilResponse", func(t *testing.T) {
		headers := headersWith("")
		body, err := client.getRequestBody(headers, &RequestConfig{})
		if err != nil {
			t.Fatalf("expected nil err when passing a nil body, got %v", err)
		}

		if body != nil {
			t.Fatal("expected nil body when passing a nil body")
		}

		if headers.Get(gomime.HEADER_CONTENT_TYPE) != "" {
			t.Fatal("expected no Content-Type for empty body")
		}
	})

	t.Run("BodyJsonResponse", func(t *testing.T) {
		body, err := client.getRequestBody(headersWith("application/json"), &RequestConfig{Body: []string{"one", "two"}})
		if err != nil {
			t.Fatalf("expected nil err when passing a json body, got %v", err)
		}

		if got := readBody(t, body); got != `["one","two"]` {
			t.Fatalf("wrong json body: %q", got)
		}
	})

	t.Run("BodyXmlResponse", func(t *testing.T) {
		body, err := client.getRequestBody(headersWith("application/xml"), &RequestConfig{Body: []string{"one", "two"}})
		if err != nil {
			t.Fatalf("expected nil err when passing a xml body, got %v", err)
		}

		if got := readBody(t, body); got != "<string>one</string><string>two</string>" {
			t.Fatalf("wrong xml body: %q", got)
		}
	})

	t.Run("XmlContentTypeWithParamsAndVariants", func(t *testing.T) {
		for _, contentType := range []string{
			"application/xml; charset=utf-8",
			"Application/XML",
			"text/xml",
			"application/atom+xml",
		} {
			body, err := client.getRequestBody(headersWith(contentType), &RequestConfig{Body: []string{"one"}})
			if err != nil {
				t.Fatalf("%s: expected nil err, got %v", contentType, err)
			}

			if got := readBody(t, body); got != "<string>one</string>" {
				t.Fatalf("%s: expected xml body, got %q", contentType, got)
			}
		}
	})

	t.Run("JsonContentTypeWithParams", func(t *testing.T) {
		headers := headersWith("application/json; charset=utf-8")
		body, err := client.getRequestBody(headers, &RequestConfig{Body: []string{"one"}})
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}

		if got := readBody(t, body); got != `["one"]` {
			t.Fatalf("expected json body, got %q", got)
		}

		if headers.Get(gomime.HEADER_CONTENT_TYPE) != "application/json; charset=utf-8" {
			t.Fatalf("expected Content-Type to stay as set, got %q", headers.Get(gomime.HEADER_CONTENT_TYPE))
		}
	})

	t.Run("BodyDefaultResponseSetsJsonContentType", func(t *testing.T) {
		headers := headersWith("")
		body, err := client.getRequestBody(headers, &RequestConfig{Body: []string{"three", "four"}})
		if err != nil {
			t.Fatalf("expected nil err when passing a json body, got %v", err)
		}

		if got := readBody(t, body); got != `["three","four"]` {
			t.Fatalf("wrong default body: %q", got)
		}

		if headers.Get(gomime.HEADER_CONTENT_TYPE) != gomime.CONTENT_TYPE_JSON {
			t.Fatalf("expected JSON Content-Type to be set, got %q", headers.Get(gomime.HEADER_CONTENT_TYPE))
		}
	})

	t.Run("RawBytesAndStringAreSentAsIs", func(t *testing.T) {
		for _, raw := range []any{[]byte("raw-bytes"), "raw-string"} {
			headers := headersWith("text/plain")
			body, err := client.getRequestBody(headers, &RequestConfig{Body: raw})
			if err != nil {
				t.Fatalf("expected nil err, got %v", err)
			}

			want := raw
			if b, ok := raw.([]byte); ok {
				want = string(b)
			}

			if got := readBody(t, body); got != want {
				t.Fatalf("expected raw body %q, got %q", want, got)
			}

			if headers.Get(gomime.HEADER_CONTENT_TYPE) != "text/plain" {
				t.Fatalf("expected Content-Type to stay as set, got %q", headers.Get(gomime.HEADER_CONTENT_TYPE))
			}
		}
	})

	t.Run("BodyConflictReturnsError", func(t *testing.T) {
		_, err := client.getRequestBody(headersWith(""), &RequestConfig{
			Body:       map[string]string{"name": "test"},
			BodyReader: strings.NewReader("raw"),
		})

		if !errors.Is(err, ErrRequestBodyConflict) {
			t.Fatalf("expected ErrRequestBodyConflict, got %v", err)
		}
	})
}

func TestDoSetsJsonContentTypeWhenMissing(t *testing.T) {
	var gotContentType, gotBody string
	client := NewBuilder().SetTransport(roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		gotContentType = request.Header.Get(gomime.HEADER_CONTENT_TYPE)
		payload, _ := io.ReadAll(request.Body)
		gotBody = string(payload)

		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("")), Request: request}, nil
	})).Build()

	if _, err := client.Post(t.Context(), "http://example.test/", WithBody(map[string]string{"a": "b"})); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if gotContentType != gomime.CONTENT_TYPE_JSON || gotBody != `{"a":"b"}` {
		t.Fatalf("expected JSON body with JSON Content-Type, got %q %q", gotContentType, gotBody)
	}
}

func TestResponseHeadersComeFromServerResponse(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			headers := make(http.Header)
			headers.Set("X-Response-Id", "resp-123")

			return &http.Response{
				Status:     "202 Accepted",
				StatusCode: http.StatusAccepted,
				Header:     headers,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    request,
			}, nil
		}),
	}

	client := NewBuilder().SetHttpClient(httpClient).Build()

	requestHeaders := make(http.Header)
	requestHeaders.Set("X-Request-Id", "req-123")

	response, err := client.Get(t.Context(), "http://example.test/resource", WithHeaders(requestHeaders))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.Headers.Get("X-Response-Id") != "resp-123" {
		t.Fatalf("expected response header, got %q", response.Headers.Get("X-Response-Id"))
	}

	if response.Headers.Get("X-Request-Id") != "" {
		t.Fatalf("did not expect request headers in response, got %q", response.Headers.Get("X-Request-Id"))
	}
}

func TestPutAndPatchSendRequestBody(t *testing.T) {
	mockTransport := gohttp_mock.NewTransport()

	headers := make(http.Header)
	headers.Set(gomime.HEADER_CONTENT_TYPE, gomime.CONTENT_TYPE_JSON)

	client := NewBuilder().SetHeaders(headers).SetTransport(mockTransport).Build()

	mockTransport.AddMock(gohttp_mock.Mock{
		Method:         http.MethodPut,
		Url:            "http://localhost/put",
		RequestBody:    `{"name":"put"}`,
		ResponseStatus: http.StatusOK,
		ResponseBody:   "put-ok",
	})

	putResponse, err := client.Put(t.Context(), "http://localhost/put", WithBody(map[string]string{"name": "put"}))
	if err != nil {
		t.Fatalf("expected nil error for PUT, got %v", err)
	}

	if putResponse.BodyString() != "put-ok" {
		t.Fatalf("expected PUT mock response, got %q", putResponse.BodyString())
	}

	mockTransport.DeleteMocks()
	mockTransport.AddMock(gohttp_mock.Mock{
		Method:         http.MethodPatch,
		Url:            "http://localhost/patch",
		RequestBody:    `{"name":"patch"}`,
		ResponseStatus: http.StatusOK,
		ResponseBody:   "patch-ok",
	})

	patchResponse, err := client.Patch(t.Context(), "http://localhost/patch", WithBody(map[string]string{"name": "patch"}))
	if err != nil {
		t.Fatalf("expected nil error for PATCH, got %v", err)
	}

	if patchResponse.BodyString() != "patch-ok" {
		t.Fatalf("expected PATCH mock response, got %q", patchResponse.BodyString())
	}
}

func TestMockResponseIncludesHTTPStatusText(t *testing.T) {
	mockTransport := gohttp_mock.NewTransport()
	mockTransport.AddMock(gohttp_mock.Mock{
		Method:         http.MethodGet,
		Url:            "http://localhost/status",
		ResponseStatus: http.StatusCreated,
		ResponseBody:   "created",
	})

	client := NewBuilder().SetTransport(mockTransport).Build()

	response, err := client.Get(t.Context(), "http://localhost/status")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.Status != "201 Created" {
		t.Fatalf("expected full HTTP status, got %q", response.Status)
	}
}

func TestBuildSnapshotsBuilderConfiguration(t *testing.T) {
	seenUserAgents := make([]string, 0, 2)
	httpClient := &http.Client{
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			seenUserAgents = append(seenUserAgents, request.Header.Get(gomime.HEADER_USER_AGENT))

			return &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader("ok")),
				Request:    request,
			}, nil
		}),
	}

	builder := NewBuilder().
		SetHttpClient(httpClient).
		SetUserAgent("first-agent")

	firstClient := builder.Build()

	builder.SetUserAgent("second-agent")
	secondClient := builder.Build()

	if _, err := firstClient.Get(t.Context(), "http://example.test/first"); err != nil {
		t.Fatalf("expected nil error for first client, got %v", err)
	}

	if _, err := secondClient.Get(t.Context(), "http://example.test/second"); err != nil {
		t.Fatalf("expected nil error for second client, got %v", err)
	}

	if len(seenUserAgents) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(seenUserAgents))
	}

	if seenUserAgents[0] != "first-agent" {
		t.Fatalf("expected first client to keep original user agent, got %q", seenUserAgents[0])
	}

	if seenUserAgents[1] != "second-agent" {
		t.Fatalf("expected second client to use updated user agent, got %q", seenUserAgents[1])
	}
}

func TestDoWithContextUsesOptions(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("expected request body to be readable, got %v", err)
			}

			if request.Header.Get("X-Request-Id") != "req-123" {
				t.Fatalf("expected request header to be set, got %q", request.Header.Get("X-Request-Id"))
			}

			if string(body) != `{"name":"codex"}` {
				t.Fatalf("expected marshalled JSON body, got %q", string(body))
			}

			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Header:        make(http.Header),
				Body:          io.NopCloser(strings.NewReader("ok")),
				ContentLength: 2,
				Request:       request,
			}, nil
		}),
	}

	headers := make(http.Header)
	headers.Set(gomime.HEADER_CONTENT_TYPE, gomime.CONTENT_TYPE_JSON)

	client := NewBuilder().SetHttpClient(httpClient).SetHeaders(headers).Build()

	requestHeaders := make(http.Header)
	requestHeaders.Set("X-Request-Id", "req-123")

	response, err := client.Do(t.Context(), http.MethodPost, "http://example.test/resource",
		WithHeaders(requestHeaders),
		WithBody(map[string]string{"name": "codex"}),
	)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.BodyString() != "ok" {
		t.Fatalf("expected ok response body, got %q", response.BodyString())
	}
}

func TestDoWithCanceledContextStopsRequest(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()

			return nil, request.Context().Err()
		}),
	}

	client := NewBuilder().SetHttpClient(httpClient).Build()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := client.Do(ctx, http.MethodGet, "http://example.test/resource")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestResponseMetadataIsPopulated(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			firstRequest := &http.Request{URL: mustParseURL(t, "http://example.test/original")}
			redirectResponse := &http.Response{
				Status:     "302 Found",
				StatusCode: http.StatusFound,
				Header:     make(http.Header),
				Request:    firstRequest,
			}

			request.Response = redirectResponse

			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Header:        make(http.Header),
				Body:          io.NopCloser(strings.NewReader("ok")),
				ContentLength: 2,
				Request:       request,
			}, nil
		}),
	}

	client := NewBuilder().SetHttpClient(httpClient).Build()

	response, err := client.Do(t.Context(), http.MethodGet, "http://example.test/final")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.FinalURL != "http://example.test/final" {
		t.Fatalf("expected final URL to be populated, got %q", response.FinalURL)
	}

	if response.ContentLength != 2 {
		t.Fatalf("expected content length 2, got %d", response.ContentLength)
	}

	if response.RedirectCount != 1 {
		t.Fatalf("expected 1 redirect, got %d", response.RedirectCount)
	}

	if response.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", response.Duration)
	}
}

func TestBuilderMaxBodyBytesLimitsResponse(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			headers := make(http.Header)
			headers.Set("X-Response-Id", "resp-123")

			firstRequest := &http.Request{URL: mustParseURL(t, "http://example.test/original")}
			redirectResponse := &http.Response{
				Status:     "302 Found",
				StatusCode: http.StatusFound,
				Header:     make(http.Header),
				Request:    firstRequest,
			}

			request.Response = redirectResponse

			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Header:        headers,
				Body:          io.NopCloser(strings.NewReader("toolarge")),
				ContentLength: int64(len("toolarge")),
				Request:       request,
			}, nil
		}),
	}

	client := NewBuilder().SetHttpClient(httpClient).SetMaxBodyBytes(4).Build()

	response, err := client.Do(t.Context(), http.MethodGet, "http://example.test/resource")
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}

	if response == nil {
		t.Fatal("expected response to be returned with ErrBodyTooLarge")
	}

	if response.StatusCode != http.StatusOK {
		t.Fatalf("expected status code 200, got %d", response.StatusCode)
	}

	if response.FinalURL != "http://example.test/resource" {
		t.Fatalf("expected final URL to be populated, got %q", response.FinalURL)
	}

	if response.Headers.Get("X-Response-Id") != "resp-123" {
		t.Fatalf("expected response headers to be preserved, got %q", response.Headers.Get("X-Response-Id"))
	}

	if response.ContentLength != int64(len("toolarge")) {
		t.Fatalf("expected content length %d, got %d", len("toolarge"), response.ContentLength)
	}

	if response.RedirectCount != 1 {
		t.Fatalf("expected 1 redirect, got %d", response.RedirectCount)
	}

	if response.Duration <= 0 {
		t.Fatalf("expected positive duration, got %v", response.Duration)
	}

	if response.BodyString() != "tool" {
		t.Fatalf("expected body truncated to limit of 4 bytes, got %q", response.BodyString())
	}
}

func TestRequestMaxBodyBytesOverridesBuilderLimit(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Header:        make(http.Header),
				Body:          io.NopCloser(strings.NewReader("123456")),
				ContentLength: 6,
				Request:       request,
			}, nil
		}),
	}

	client := NewBuilder().SetHttpClient(httpClient).SetMaxBodyBytes(3).Build()

	response, err := client.Do(t.Context(), http.MethodGet, "http://example.test/resource", WithMaxBodyBytes(6))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.BodyString() != "123456" {
		t.Fatalf("expected full body to be read, got %q", response.BodyString())
	}
}

func TestSetTransportUsesCustomRoundTripper(t *testing.T) {
	client := NewBuilder().SetTransport(roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:        "200 OK",
			StatusCode:    http.StatusOK,
			Header:        make(http.Header),
			Body:          io.NopCloser(strings.NewReader("transport-ok")),
			ContentLength: int64(len("transport-ok")),
			Request:       request,
		}, nil
	})).Build()

	response, err := client.Do(t.Context(), http.MethodGet, "http://example.test/resource")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.BodyString() != "transport-ok" {
		t.Fatalf("expected custom transport response, got %q", response.BodyString())
	}
}

func TestSetHttpClientHasPriorityOverBuilderTransport(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Header:        make(http.Header),
				Body:          io.NopCloser(strings.NewReader("http-client-wins")),
				ContentLength: int64(len("http-client-wins")),
				Request:       request,
			}, nil
		}),
	}

	client := NewBuilder().
		SetHttpClient(httpClient).
		SetTransport(roundTripperFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Header:        make(http.Header),
				Body:          io.NopCloser(strings.NewReader("builder-transport")),
				ContentLength: int64(len("builder-transport")),
				Request:       request,
			}, nil
		})).
		Build()

	response, err := client.Do(t.Context(), http.MethodGet, "http://example.test/resource")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.BodyString() != "http-client-wins" {
		t.Fatalf("expected provided http client to win, got %q", response.BodyString())
	}
}

func TestRedirectPolicyStopsAfterLimit(t *testing.T) {
	redirectCalls := 0
	client := NewBuilder().SetRedirectPolicy(RedirectPolicy{
		MaxRedirects: 1,
		Validate: func(req *http.Request, via []*http.Request) error {
			redirectCalls++
			return nil
		},
	}).Build()

	concreteClient, ok := client.(*Client)
	if !ok {
		t.Fatal("expected concrete client instance")
	}

	checkRedirect := concreteClient.getCheckRedirect()
	if checkRedirect == nil {
		t.Fatal("expected redirect checker")
	}

	err := checkRedirect(&http.Request{}, []*http.Request{{}})
	if err != nil {
		t.Fatalf("expected first redirect to be allowed, got %v", err)
	}

	if redirectCalls != 1 {
		t.Fatalf("expected validate hook to be called for allowed redirect, got %d calls", redirectCalls)
	}

	err = checkRedirect(&http.Request{}, []*http.Request{{}, {}})
	if !errors.Is(err, ErrTooManyRedirects) || !IsRedirectError(err) {
		t.Fatalf("expected ErrTooManyRedirects after limit, got %v", err)
	}

	if IsRedirectError(http.ErrUseLastResponse) {
		t.Fatal("expected ErrUseLastResponse not to be reported as redirect error")
	}

	if redirectCalls != 1 {
		t.Fatalf("expected validate hook to be skipped when limit is exceeded, got %d calls", redirectCalls)
	}
}

func newRedirectLoopServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	requests := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.Redirect(w, r, "/", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	return server, requests
}

func TestRedirectLimitReturnsErrorWithAndWithoutPolicy(t *testing.T) {
	builders := map[string]ClientBuilder{
		"no policy": NewBuilder(),
		"policy with zero limit": NewBuilder().SetRedirectPolicy(RedirectPolicy{
			Validate: func(req *http.Request, via []*http.Request) error {
				return nil
			},
		}),
	}

	for name, builder := range builders {
		t.Run(name, func(t *testing.T) {
			server, requests := newRedirectLoopServer(t)

			response, err := builder.Build().Get(t.Context(), server.URL)
			if !errors.Is(err, ErrTooManyRedirects) || !IsRedirectError(err) {
				t.Fatalf("expected ErrTooManyRedirects, got %v", err)
			}

			if response != nil {
				t.Fatalf("expected no response with redirect error, got %d", response.StatusCode)
			}

			if got := requests.Load(); got != DEFAULT_MAX_REDIRECTS+1 {
				t.Fatalf("expected %d requests, got %d", DEFAULT_MAX_REDIRECTS+1, got)
			}
		})
	}
}

func TestRedirectPolicyNegativeLimitDoesNotFollow(t *testing.T) {
	server, requests := newRedirectLoopServer(t)

	validateCalls := 0
	client := NewBuilder().SetRedirectPolicy(RedirectPolicy{
		MaxRedirects: -1,
		Validate: func(req *http.Request, via []*http.Request) error {
			validateCalls++
			return nil
		},
	}).Build()

	response, err := client.Get(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("expected first redirect response without error, got %v", err)
	}

	if response.StatusCode != http.StatusFound || response.RedirectCount != 0 {
		t.Fatalf("expected first 302 without redirects, got %d after %d redirects", response.StatusCode, response.RedirectCount)
	}

	if requests.Load() != 1 || validateCalls != 0 {
		t.Fatalf("expected one request and no validate calls, got %d requests and %d calls", requests.Load(), validateCalls)
	}
}

func TestSetCheckRedirectIsUsed(t *testing.T) {
	expectedErr := errors.New("stop redirect")
	client := NewBuilder().SetCheckRedirect(func(req *http.Request, via []*http.Request) error {
		return expectedErr
	}).Build()

	concreteClient, ok := client.(*Client)
	if !ok {
		t.Fatal("expected concrete client instance")
	}

	err := concreteClient.getCheckRedirect()(&http.Request{}, nil)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected custom redirect error, got %v", err)
	}
}

func TestErrorHelpers(t *testing.T) {
	dnsErr := &url.Error{Err: &net.DNSError{Err: "lookup failed"}}
	if !IsDNS(dnsErr) || ClassifyError(dnsErr) != ErrorKindDNS {
		t.Fatalf("expected DNS classification, got %q", ClassifyError(dnsErr))
	}

	timeoutErr := &url.Error{Err: timeoutNetError{}}
	if !IsTimeout(timeoutErr) || ClassifyError(timeoutErr) != ErrorKindTimeout {
		t.Fatalf("expected timeout classification, got %q", ClassifyError(timeoutErr))
	}

	tlsErr := &url.Error{Err: tls.RecordHeaderError{}}
	if !IsTLS(tlsErr) || ClassifyError(tlsErr) != ErrorKindTLS {
		t.Fatalf("expected TLS classification, got %q", ClassifyError(tlsErr))
	}

	connectionErr := &url.Error{Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}}
	if !IsConnection(connectionErr) || ClassifyError(connectionErr) != ErrorKindConnection {
		t.Fatalf("expected connection classification, got %q", ClassifyError(connectionErr))
	}

	if !IsCanceled(context.Canceled) || ClassifyError(context.Canceled) != ErrorKindCanceled {
		t.Fatalf("expected canceled classification, got %q", ClassifyError(context.Canceled))
	}
}

func TestMockTransportCanBeUsedViaBuilder(t *testing.T) {
	mockTransport := gohttp_mock.NewTransport()
	mockTransport.AddMock(gohttp_mock.Mock{
		Method:         http.MethodPost,
		Url:            "http://example.test/mock",
		RequestBody:    `{"name":"builder-mock"}`,
		ResponseStatus: http.StatusAccepted,
		ResponseHeaders: http.Header{
			"X-Mock": []string{"true"},
		},
		ResponseBody: "accepted",
	})

	headers := make(http.Header)
	headers.Set(gomime.HEADER_CONTENT_TYPE, gomime.CONTENT_TYPE_JSON)

	client := NewBuilder().SetHeaders(headers).SetTransport(mockTransport).Build()

	response, err := client.Do(t.Context(), http.MethodPost, "http://example.test/mock", WithBody(map[string]string{"name": "builder-mock"}))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("expected accepted status, got %d", response.StatusCode)
	}

	if response.Headers.Get("X-Mock") != "true" {
		t.Fatalf("expected mock header, got %q", response.Headers.Get("X-Mock"))
	}
}

func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("expected valid test URL, got %v", err)
	}

	return parsedURL
}

type timeoutNetError struct{}

func (timeoutNetError) Error() string   { return "timeout" }
func (timeoutNetError) Timeout() bool   { return true }
func (timeoutNetError) Temporary() bool { return true }

func TestDeadlineErrorsAreClassifiedAsTimeout(t *testing.T) {
	if IsCanceled(context.DeadlineExceeded) {
		t.Fatal("expected deadline exceeded not to be treated as canceled")
	}

	if ClassifyError(context.DeadlineExceeded) != ErrorKindTimeout {
		t.Fatalf("expected timeout classification, got %q", ClassifyError(context.DeadlineExceeded))
	}

	canceledErr := &url.Error{Op: "Get", URL: "http://example.test", Err: context.Canceled}
	if ClassifyError(canceledErr) != ErrorKindCanceled {
		t.Fatalf("expected wrapped cancel to be canceled, got %q", ClassifyError(canceledErr))
	}

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer server.Close()
	defer close(release)

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	_, err := NewBuilder().Build().Get(ctx, server.URL)
	if ClassifyError(err) != ErrorKindTimeout {
		t.Fatalf("expected context deadline to be classified as timeout, got %q (%v)", ClassifyError(err), err)
	}
}

func TestDefaultTransportKeepsStandardSettings(t *testing.T) {
	client := NewBuilder().SetMaxIdleConnections(7).Build().(*Client)
	httpClient := client.getHttpClient()

	transport, ok := httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected *http.Transport, got %T", httpClient.Transport)
	}

	if transport.Proxy == nil {
		t.Fatal("expected proxy from environment to be kept")
	}

	if !transport.ForceAttemptHTTP2 {
		t.Fatal("expected HTTP/2 to stay enabled")
	}

	if transport.TLSHandshakeTimeout == 0 || transport.IdleConnTimeout == 0 {
		t.Fatal("expected TLS handshake and idle connection timeouts to be set")
	}

	if transport.MaxIdleConns != 7 || transport.MaxIdleConnsPerHost != 7 {
		t.Fatalf("expected idle connection limits of 7, got %d/%d", transport.MaxIdleConns, transport.MaxIdleConnsPerHost)
	}

	if transport.ResponseHeaderTimeout != DEFAULT_RESPONSE_TIMEOUT {
		t.Fatalf("expected default response header timeout, got %v", transport.ResponseHeaderTimeout)
	}

	if httpClient.Timeout != DEFAULT_REQUEST_TIMEOUT {
		t.Fatalf("expected default whole-request timeout, got %v", httpClient.Timeout)
	}

	if http.DefaultTransport.(*http.Transport) == transport {
		t.Fatal("expected default transport to be cloned, not shared")
	}
}

func TestResponseTimeoutDoesNotLimitBodyReading(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(time.Second)
		_, _ = w.Write([]byte("slow-body"))
	}))
	defer server.Close()

	client := NewBuilder().
		SetConnectionTimeout(300 * time.Millisecond).
		SetResponseTimeout(300 * time.Millisecond).
		Build()

	response, err := client.Get(t.Context(), server.URL)
	if err != nil {
		t.Fatalf("expected slow body to be read after headers arrived, got %v", err)
	}

	if response.BodyString() != "slow-body" {
		t.Fatalf("expected full body, got %q", response.BodyString())
	}

	limited := NewBuilder().SetRequestTimeout(300 * time.Millisecond).Build()

	_, err = limited.Get(t.Context(), server.URL)
	if ClassifyError(err) != ErrorKindTimeout {
		t.Fatalf("expected request timeout to stop slow body, got %q (%v)", ClassifyError(err), err)
	}
}

func TestTLSAlertFromServerIsClassifiedAsTLS(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
	server.StartTLS()
	defer server.Close()

	for _, maxVersion := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
		httpClient := server.Client()
		httpClient.Transport.(*http.Transport).TLSClientConfig.MaxVersion = maxVersion

		_, err := NewBuilder().SetHttpClient(httpClient).Build().Get(t.Context(), server.URL)
		if err == nil {
			t.Fatalf("TLS %x: expected handshake error without client certificate", maxVersion)
		}

		if !IsTLS(err) || ClassifyError(err) != ErrorKindTLS {
			t.Fatalf("TLS %x: expected TLS classification, got %q (%v)", maxVersion, ClassifyError(err), err)
		}

		if IsConnection(err) {
			t.Fatalf("TLS %x: expected TLS alert not to be a connection error (%v)", maxVersion, err)
		}
	}
}

func TestBuildClonesRedirectPolicyAndHeaders(t *testing.T) {
	headers := http.Header{"X-Common": {"one"}}
	builder := NewBuilder().SetHeaders(headers).SetRedirectPolicy(RedirectPolicy{MaxRedirects: 2})

	client := builder.Build().(*Client)

	headers.Set("X-Common", "changed")
	builder.SetRedirectPolicy(RedirectPolicy{MaxRedirects: 5})

	if client.builder.headers.Get("X-Common") != "one" {
		t.Fatalf("expected built client to keep its headers, got %q", client.builder.headers.Get("X-Common"))
	}

	if client.builder.redirectPolicy.MaxRedirects != 2 {
		t.Fatalf("expected built client to keep its redirect policy, got %d", client.builder.redirectPolicy.MaxRedirects)
	}
}

func TestRequestTimeoutDefaultsAndOverrides(t *testing.T) {
	cases := map[string]struct {
		builder ClientBuilder
		want    time.Duration
	}{
		"default":                        {NewBuilder(), DEFAULT_REQUEST_TIMEOUT},
		"custom":                         {NewBuilder().SetRequestTimeout(2 * time.Second), 2 * time.Second},
		"disabled":                       {NewBuilder().DisableTimeouts(true), 0},
		"disabled with explicit":         {NewBuilder().DisableTimeouts(true).SetRequestTimeout(time.Second), time.Second},
		"custom transport keeps default": {NewBuilder().SetTransport(http.DefaultTransport), DEFAULT_REQUEST_TIMEOUT},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := tc.builder.Build().(*Client).getHttpClient().Timeout
			if got != tc.want {
				t.Fatalf("expected request timeout %v, got %v", tc.want, got)
			}
		})
	}
}

func TestGetRequestBodyReaderFormAndUnsupportedTypes(t *testing.T) {
	client := Client{}

	read := func(t *testing.T, body io.Reader) string {
		t.Helper()

		payload, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("expected readable body, got %v", err)
		}

		return string(payload)
	}

	t.Run("ReaderIsSentAsIs", func(t *testing.T) {
		headers := http.Header{}
		body, err := client.getRequestBody(headers, &RequestConfig{Body: bytes.NewBufferString("x")})
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}

		if got := read(t, body); got != "x" {
			t.Fatalf("expected raw reader body, got %q", got)
		}

		if headers.Get(gomime.HEADER_CONTENT_TYPE) != "" {
			t.Fatalf("expected no Content-Type for reader body, got %q", headers.Get(gomime.HEADER_CONTENT_TYPE))
		}
	})

	t.Run("FormValuesWithoutContentType", func(t *testing.T) {
		headers := http.Header{}
		body, err := client.getRequestBody(headers, &RequestConfig{Body: url.Values{"a": {"1", "2"}, "b": {"x y"}}})
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}

		if got := read(t, body); got != "a=1&a=2&b=x+y" {
			t.Fatalf("expected form body, got %q", got)
		}

		if headers.Get(gomime.HEADER_CONTENT_TYPE) != gomime.CONTENT_TYPE_FORM {
			t.Fatalf("expected form Content-Type, got %q", headers.Get(gomime.HEADER_CONTENT_TYPE))
		}
	})

	t.Run("FormValuesWithFormContentType", func(t *testing.T) {
		headers := http.Header{gomime.HEADER_CONTENT_TYPE: {"application/x-www-form-urlencoded; charset=utf-8"}}
		body, err := client.getRequestBody(headers, &RequestConfig{Body: url.Values{"a": {"1"}}})
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}

		if got := read(t, body); got != "a=1" {
			t.Fatalf("expected form body, got %q", got)
		}
	})

	t.Run("JsonSuffixMediaType", func(t *testing.T) {
		headers := http.Header{gomime.HEADER_CONTENT_TYPE: {"application/problem+json"}}
		body, err := client.getRequestBody(headers, &RequestConfig{Body: map[string]int{"a": 1}})
		if err != nil {
			t.Fatalf("expected nil err, got %v", err)
		}

		if got := read(t, body); got != `{"a":1}` {
			t.Fatalf("expected json body, got %q", got)
		}
	})

	for name, contentType := range map[string]string{
		"FormContentTypeWithStruct": gomime.CONTENT_TYPE_FORM,
		"UnknownContentType":        "text/plain",
	} {
		t.Run(name, func(t *testing.T) {
			headers := http.Header{gomime.HEADER_CONTENT_TYPE: {contentType}}
			_, err := client.getRequestBody(headers, &RequestConfig{Body: map[string]string{"a": "b"}})
			if !errors.Is(err, ErrUnsupportedContentType) {
				t.Fatalf("expected ErrUnsupportedContentType, got %v", err)
			}
		})
	}
}

func TestMaxBodyBytesDefaultsAndOverrides(t *testing.T) {
	cases := map[string]struct {
		builderLimit int64
		requestLimit int64
		want         int64
	}{
		"default":                      {0, 0, DEFAULT_MAX_BODY_BYTES},
		"builder limit":                {5, 0, 5},
		"request overrides builder":    {5, 9, 9},
		"builder unlimited":            {-1, 0, 0},
		"request unlimited":            {5, -1, 0},
		"request limit over unlimited": {-1, 9, 9},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			client := NewBuilder().SetMaxBodyBytes(tc.builderLimit).Build().(*Client)

			if got := client.getMaxBodyBytes(&RequestConfig{MaxBodyBytes: tc.requestLimit}); got != tc.want {
				t.Fatalf("expected limit %d, got %d", tc.want, got)
			}
		})
	}
}

func TestResponseBodyIsBoundedByDefault(t *testing.T) {
	size := DEFAULT_MAX_BODY_BYTES + 1024
	client := NewBuilder().SetTransport(roundTripperFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(make([]byte, size))),
			Request:    request,
		}, nil
	})).Build()

	response, err := client.Get(t.Context(), "http://example.test/big")
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
	}

	if len(response.Body) != DEFAULT_MAX_BODY_BYTES {
		t.Fatalf("expected body truncated to default limit, got %d bytes", len(response.Body))
	}

	unlimited, err := client.Get(t.Context(), "http://example.test/big", WithMaxBodyBytes(-1))
	if err != nil || len(unlimited.Body) != size {
		t.Fatalf("expected full %d bytes without error, got %d bytes, err %v", size, len(unlimited.Body), err)
	}
}
