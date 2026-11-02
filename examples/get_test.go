package examples

import (
	"errors"
	"net/http"
	"testing"

	"github.com/georgebent/go-httpclient/gohttp_mock"
)

func useMocks(t *testing.T, mocks ...gohttp_mock.Mock) {
	t.Helper()

	transport := gohttp_mock.NewTransport()
	for _, mock := range mocks {
		transport.AddMock(mock)
	}

	original := HttpClient
	HttpClient = newHttpClient(transport)
	t.Cleanup(func() { HttpClient = original })
}

func TestGetError(t *testing.T) {
	errTimeout := errors.New("timeout getting response")

	useMocks(t, gohttp_mock.Mock{
		Method: http.MethodGet,
		Url:    "http://localhost",
		Error:  errTimeout,
	})

	result, err := Get()
	if result != "" {
		t.Error("Expect empty body")
	}

	if !errors.Is(err, errTimeout) {
		t.Fatalf("Expect mocked timeout error, got %v", err)
	}
}

func TestGetResponse(t *testing.T) {
	useMocks(t, gohttp_mock.Mock{
		Method:         http.MethodGet,
		Url:            "http://localhost",
		ResponseStatus: http.StatusOK,
		ResponseBody:   "Ok",
	})

	result, err := Get()
	if err != nil {
		t.Fatalf("Expect empty error, got %v", err)
	}

	if result != "Ok" {
		t.Fatalf("Expect message 'Ok', got %q", result)
	}
}
