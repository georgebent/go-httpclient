package gohttp_mock

import "net/http"

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
