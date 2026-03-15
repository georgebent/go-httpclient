package gohttp

import (
	"io"
	"net/http"
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
		body, err := client.getRequestBody("", nil)

		if err != nil {
			t.Error("Expected nil err when passing a nil body")
		}

		if body != nil {
			t.Error("Expected nil body when passing a nil body")
		}
	})

	t.Run("BodyJsonResponse", func(t *testing.T) {
		requestBody := []string{"one", "two"}

		body, err := client.getRequestBody("application/json", requestBody)
		if err != nil {
			t.Error("Expected nil err when passing a json body")
		}

		if string(body) != `["one","two"]` {
			t.Error("Wrong body when passing a json body")
		}
	})

	t.Run("BodyXmlResponse", func(t *testing.T) {
		requestBody := []string{"one", "two"}

		body, err := client.getRequestBody("application/xml", requestBody)
		if err != nil {
			t.Error("Expected nil err when passing a json body")
		}

		if string(body) != "<string>one</string><string>two</string>" {
			t.Error("Wrong body when passing a json body")
		}
	})

	t.Run("BodyDefaultResponse", func(t *testing.T) {
		requestBody := []string{"three", "four"}

		body, err := client.getRequestBody("", requestBody)
		if err != nil {
			t.Error("Expected nil err when passing a json body")
		}

		if string(body) != `["three","four"]` {
			t.Error("Wrong body when passing a json body")
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

	response, err := client.Get("http://example.test/resource", requestHeaders)
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

	putResponse, err := client.Put("http://localhost/put", nil, map[string]string{"name": "put"})
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

	patchResponse, err := client.Patch("http://localhost/patch", nil, map[string]string{"name": "patch"})
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

	response, err := client.Get("http://localhost/status", nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if response.Status != "201 Created" {
		t.Fatalf("expected full HTTP status, got %q", response.Status)
	}
}
