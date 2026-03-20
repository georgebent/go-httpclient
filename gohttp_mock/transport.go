package gohttp_mock

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

type MockTransport struct {
	mutex sync.RWMutex
	mocks map[string]*Mock
}

func NewTransport() *MockTransport {
	return &MockTransport{
		mocks: make(map[string]*Mock),
	}
}

func (t *MockTransport) AddMock(mock Mock) {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.mocks[t.getMockKey(mock.Method, mock.Url, mock.RequestBody)] = &mock
}

func (t *MockTransport) DeleteMocks() {
	t.mutex.Lock()
	defer t.mutex.Unlock()

	t.mocks = make(map[string]*Mock)
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

	responseHeaders := make(http.Header)
	for key, values := range mock.ResponseHeaders {
		responseHeaders[key] = append([]string(nil), values...)
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

	return t.mocks[t.getMockKey(method, url, body)]
}

func (t *MockTransport) getMockKey(method string, url string, body string) string {
	hasher := md5.New()
	hasher.Write([]byte(method + url + cleanBody(body)))

	return hex.EncodeToString(hasher.Sum(nil))
}

func cleanBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	body = strings.ReplaceAll(body, "\t", "")
	body = strings.ReplaceAll(body, "\n", "")

	return body
}
