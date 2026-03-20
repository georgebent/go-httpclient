package gohttp

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

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

func TestGetRequestBody(t *testing.T) {
	client := Client{}

	t.Run("noBodyNilResponse", func(t *testing.T) {
		body, err := client.getRequestBody("", &RequestConfig{})

		if err != nil {
			t.Error("Expected nil err when passing a nil body")
		}

		if body != nil {
			t.Error("Expected nil body when passing a nil body")
		}
	})

	t.Run("BodyJsonResponse", func(t *testing.T) {
		requestBody := []string{"one", "two"}

		body, err := client.getRequestBody("application/json", &RequestConfig{Body: requestBody})
		if err != nil {
			t.Error("Expected nil err when passing a json body")
		}

		payload, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("expected readable body, got %v", err)
		}

		if string(payload) != `["one","two"]` {
			t.Error("Wrong body when passing a json body")
		}
	})

	t.Run("BodyXmlResponse", func(t *testing.T) {
		requestBody := []string{"one", "two"}

		body, err := client.getRequestBody("application/xml", &RequestConfig{Body: requestBody})
		if err != nil {
			t.Error("Expected nil err when passing a json body")
		}

		payload, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("expected readable body, got %v", err)
		}

		if string(payload) != "<string>one</string><string>two</string>" {
			t.Error("Wrong body when passing a json body")
		}
	})

	t.Run("BodyDefaultResponse", func(t *testing.T) {
		requestBody := []string{"three", "four"}

		body, err := client.getRequestBody("", &RequestConfig{Body: requestBody})
		if err != nil {
			t.Error("Expected nil err when passing a json body")
		}

		payload, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("expected readable body, got %v", err)
		}

		if string(payload) != `["three","four"]` {
			t.Error("Wrong body when passing a json body")
		}
	})

	t.Run("BodyConflictReturnsError", func(t *testing.T) {
		_, err := client.getRequestBody("", &RequestConfig{
			Body:       map[string]string{"name": "test"},
			BodyReader: strings.NewReader("raw"),
		})

		if err != ErrRequestBodyConflict {
			t.Fatalf("expected ErrRequestBodyConflict, got %v", err)
		}
	})
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

	response, err := client.Get(context.Background(), "http://example.test/resource", WithHeaders(requestHeaders))
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
	gohttp_mock.StartMockServer()
	defer gohttp_mock.StopMockServer()
	gohttp_mock.MockupServer.DeleteMocks()

	headers := make(http.Header)
	headers.Set(gomime.HEADER_CONTENT_TYPE, gomime.CONTENT_TYPE_JSON)

	client := NewBuilder().SetHeaders(headers).Build()

	gohttp_mock.AddMock(gohttp_mock.Mock{
		Method:         http.MethodPut,
		Url:            "http://localhost/put",
		RequestBody:    `{"name":"put"}`,
		ResponseStatus: http.StatusOK,
		ResponseBody:   "put-ok",
	})

	putResponse, err := client.Put(context.Background(), "http://localhost/put", WithBody(map[string]string{"name": "put"}))
	if err != nil {
		t.Fatalf("expected nil error for PUT, got %v", err)
	}

	if putResponse.BodyString() != "put-ok" {
		t.Fatalf("expected PUT mock response, got %q", putResponse.BodyString())
	}

	gohttp_mock.MockupServer.DeleteMocks()
	gohttp_mock.AddMock(gohttp_mock.Mock{
		Method:         http.MethodPatch,
		Url:            "http://localhost/patch",
		RequestBody:    `{"name":"patch"}`,
		ResponseStatus: http.StatusOK,
		ResponseBody:   "patch-ok",
	})

	patchResponse, err := client.Patch(context.Background(), "http://localhost/patch", WithBody(map[string]string{"name": "patch"}))
	if err != nil {
		t.Fatalf("expected nil error for PATCH, got %v", err)
	}

	if patchResponse.BodyString() != "patch-ok" {
		t.Fatalf("expected PATCH mock response, got %q", patchResponse.BodyString())
	}
}

func TestMockResponseIncludesHTTPStatusText(t *testing.T) {
	gohttp_mock.StartMockServer()
	defer gohttp_mock.StopMockServer()
	gohttp_mock.MockupServer.DeleteMocks()

	gohttp_mock.AddMock(gohttp_mock.Mock{
		Method:         http.MethodGet,
		Url:            "http://localhost/status",
		ResponseStatus: http.StatusCreated,
		ResponseBody:   "created",
	})

	client := NewBuilder().Build()

	response, err := client.Get(context.Background(), "http://localhost/status")
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

	if _, err := firstClient.Get(context.Background(), "http://example.test/first"); err != nil {
		t.Fatalf("expected nil error for first client, got %v", err)
	}

	if _, err := secondClient.Get(context.Background(), "http://example.test/second"); err != nil {
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

	response, err := client.Do(context.Background(), http.MethodPost, "http://example.test/resource",
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

	ctx, cancel := context.WithCancel(context.Background())
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

	response, err := client.Do(context.Background(), http.MethodGet, "http://example.test/final")
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
			return &http.Response{
				Status:        "200 OK",
				StatusCode:    http.StatusOK,
				Header:        make(http.Header),
				Body:          io.NopCloser(strings.NewReader("toolarge")),
				ContentLength: int64(len("toolarge")),
				Request:       request,
			}, nil
		}),
	}

	client := NewBuilder().SetHttpClient(httpClient).SetMaxBodyBytes(4).Build()

	_, err := client.Do(context.Background(), http.MethodGet, "http://example.test/resource")
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("expected ErrBodyTooLarge, got %v", err)
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

	response, err := client.Do(context.Background(), http.MethodGet, "http://example.test/resource", WithMaxBodyBytes(6))
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

	response, err := client.Do(context.Background(), http.MethodGet, "http://example.test/resource")
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

	response, err := client.Do(context.Background(), http.MethodGet, "http://example.test/resource")
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
	if !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("expected redirect limiter to stop redirects after limit, got %v", err)
	}

	if redirectCalls != 1 {
		t.Fatalf("expected validate hook to be skipped when limit is exceeded, got %d calls", redirectCalls)
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

	response, err := client.Do(context.Background(), http.MethodPost, "http://example.test/mock", WithBody(map[string]string{"name": "builder-mock"}))
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
