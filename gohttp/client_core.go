package gohttp

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/georgebent/go-httpclient/core"
	"github.com/georgebent/go-httpclient/gohttp_mock"
	"github.com/georgebent/go-httpclient/gomime"
)

const (
	DEFAULT_MAX_IDDLE_CONNECTIONS = 5
	DEFAULT_RESPONSE_TIMEOUT      = 5 * time.Second
	DEFAULT_CONNECTION_TIMEOUT    = 1 * time.Second
)

func (c *Client) do(method string, url string, headers http.Header, body interface{}) (*core.Response, error) {
	fullHeaders := c.getRequestHeaders(headers)
	requestBody, err := c.getRequestBody(fullHeaders.Get(gomime.HEADER_CONTENT_TYPE), body)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequest(method, url, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}

	request.Header = fullHeaders

	response, err := c.getHttpClient().Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	finalResponse := core.Response{
		Status:     response.Status,
		StatusCode: response.StatusCode,
		Headers:    response.Header,
		Body:       responseBody,
	}

	return &finalResponse, nil
}

func (c *Client) getRequestHeaders(requestHeaders http.Header) http.Header {
	result := make(http.Header)

	for header, value := range c.builder.headers {
		if len(value) > 0 {
			result.Set(header, value[0])
		}
	}

	for header, value := range requestHeaders {
		if len(value) > 0 {
			result.Set(header, value[0])
		}
	}

	if c.builder.userAgent != "" {
		if result.Get(gomime.HEADER_USER_AGENT) != "" {
			return result
		}

		result.Set(gomime.HEADER_USER_AGENT, c.builder.userAgent)
	}

	return result
}

func (c *Client) getRequestBody(contentType string, body interface{}) ([]byte, error) {
	if body == nil {
		return nil, nil
	}

	switch strings.ToLower(contentType) {
	case gomime.CONTENT_TYPE_JSON:
		return json.Marshal(body)
	case gomime.CONTENT_TYPE_XML:
		return xml.Marshal(body)
	default:
		return json.Marshal(body)
	}
}

func (c *Client) getHttpClient() core.HttpClient {
	if gohttp_mock.MockupServer.IsMockServerEnabled() {
		return gohttp_mock.MockupServer.GetMockedClient()
	}

	c.clientOnce.Do(func() {
		if c.builder.client != nil {
			c.coreClient = c.builder.client
			return
		}
		c.coreClient = &http.Client{
			Timeout: c.getConnectionTimeout() + c.getResponseTimeout(),
			Transport: &http.Transport{
				MaxIdleConns:          c.getMaxIdleConnections(),
				ResponseHeaderTimeout: c.getResponseTimeout(),
				DialContext: (&net.Dialer{
					Timeout: c.getConnectionTimeout(),
				}).DialContext,
			},
		}
	})

	return c.coreClient
}

func (c *Client) getMaxIdleConnections() int {
	if c.builder.maxIdleConnections > 0 {
		return c.builder.maxIdleConnections
	}

	return DEFAULT_MAX_IDDLE_CONNECTIONS
}

func (c *Client) getResponseTimeout() time.Duration {
	if c.builder.responseTimeout > 0 {
		return c.builder.responseTimeout
	}

	if c.builder.disabledTimeouts {
		return 0
	}

	return DEFAULT_RESPONSE_TIMEOUT
}

func (c *Client) getConnectionTimeout() time.Duration {
	if c.builder.connectionTimeout > 0 {
		return c.builder.connectionTimeout
	}

	if c.builder.disabledTimeouts {
		return 0
	}

	return DEFAULT_CONNECTION_TIMEOUT
}
