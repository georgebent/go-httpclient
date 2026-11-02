package gohttp_mock

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type mockKey struct {
	method string
	url    string
	body   string
}

type MockTransport struct {
	mutex sync.RWMutex
	mocks map[mockKey]*Mock
}

func NewTransport() *MockTransport {
	return &MockTransport{
		mocks: make(map[mockKey]*Mock),
	}
}

func (t *MockTransport) AddMock(mock Mock) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.mocks[newMockKey(mock.Method, mock.Url, mock.RequestBody)] = &mock
}

func (t *MockTransport) DeleteMocks() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	clear(t.mocks)
}

func (t *MockTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := readRequestBody(request)
	if err != nil {
		return nil, err
	}

	mock := t.getMock(request.Method, request.URL.String(), string(body))
	if mock == nil {
		return nil, fmt.Errorf("no mock matching %s from '%s' with given body", request.Method, request.URL.String())
	}

	if mock.Error != nil {
		return nil, mock.Error
	}

	responseHeaders := mock.ResponseHeaders.Clone()
	if responseHeaders == nil {
		responseHeaders = make(http.Header)
	}

	return &http.Response{
		Status:        fmt.Sprintf("%d %s", mock.ResponseStatus, http.StatusText(mock.ResponseStatus)),
		StatusCode:    mock.ResponseStatus,
		Header:        responseHeaders,
		Body:          io.NopCloser(strings.NewReader(mock.ResponseBody)),
		ContentLength: int64(len(mock.ResponseBody)),
		Request:       request,
	}, nil
}

func readRequestBody(request *http.Request) ([]byte, error) {
	if request == nil {
		return nil, nil
	}

	if request.Body != nil {
		defer request.Body.Close()
	}

	if request.GetBody != nil {
		requestBody, err := request.GetBody()
		if err != nil {
			return nil, err
		}

		defer requestBody.Close()

		return io.ReadAll(requestBody)
	}

	if request.Body == nil {
		return nil, nil
	}

	return io.ReadAll(request.Body)
}

func (t *MockTransport) getMock(method string, url string, body string) *Mock {
	t.mutex.RLock()
	defer t.mutex.RUnlock()

	return t.mocks[newMockKey(method, url, body)]
}

func newMockKey(method string, url string, body string) mockKey {
	return mockKey{
		method: method,
		url:    url,
		body:   cleanBody(body),
	}
}

func cleanBody(body string) string {
	return strings.NewReplacer("\t", "", "\n", "").Replace(strings.TrimSpace(body))
}
