package gohttp_mock

import "net/http"

type httpClientMock struct {
	transport *MockTransport
}

func (c *httpClientMock) Do(request *http.Request) (*http.Response, error) {
	return c.transport.RoundTrip(request)
}
