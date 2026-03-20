package gohttp_mock

import (
	"fmt"
	"net/http"

	"github.com/georgebent/go-httpclient/core"
)

// Mock structure provides a clean way to configure HTTP mocks based on
// the combination between request method, URL and request body.
type Mock struct {
	Method      string
	Url         string
	RequestBody string
	Error       error

	ResponseHeaders http.Header
	ResponseBody    string
	ResponseStatus  int
}

// GetResponse returns a Response object based on the mock configuration.
func (m *Mock) GetResponse() (*core.Response, error) {
	if m.Error != nil {
		return nil, m.Error
	}

	response := core.Response{
		StatusCode:    m.ResponseStatus,
		Body:          []byte(m.ResponseBody),
		Status:        fmt.Sprintf("%d %s", m.ResponseStatus, http.StatusText(m.ResponseStatus)),
		Headers:       m.ResponseHeaders,
		ContentLength: int64(len(m.ResponseBody)),
	}

	return &response, nil
}
