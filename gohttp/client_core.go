package gohttp

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
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

var (
	ErrRequestBodyConflict = errors.New("request body and body reader are mutually exclusive")
	ErrUnsupportedMethod   = errors.New("unsupported HTTP method")
	ErrBodyTooLarge        = errors.New("response body exceeds limit")
)

func (c *Client) do(ctx context.Context, method string, url string, opts ...RequestOption) (*core.Response, error) {
	requestConfig, err := newRequestConfig(opts...)
	if err != nil {
		return nil, err
	}

	if ctx == nil {
		ctx = context.Background()
	}

	fullHeaders := c.getRequestHeaders(requestConfig.Headers)
	requestBody, err := c.getRequestBody(fullHeaders.Get(gomime.HEADER_CONTENT_TYPE), requestConfig)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, method, url, requestBody)
	if err != nil {
		return nil, err
	}

	request.Header = fullHeaders

	startedAt := time.Now()
	response, err := c.getHttpClient().Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()
	finalResponse := &core.Response{
		Status:        response.Status,
		StatusCode:    response.StatusCode,
		Headers:       response.Header,
		FinalURL:      c.getFinalURL(response),
		ContentLength: response.ContentLength,
		RedirectCount: c.getRedirectCount(response),
	}

	responseBody, tooLarge, err := c.readResponseBody(response.Body, c.getMaxBodyBytes(requestConfig))
	finalResponse.Body = responseBody
	finalResponse.Duration = time.Since(startedAt)
	if err != nil {
		return nil, err
	}

	if tooLarge {
		return finalResponse, ErrBodyTooLarge
	}

	return finalResponse, nil
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

func (c *Client) getRequestBody(contentType string, cfg *RequestConfig) (io.Reader, error) {
	if cfg.BodyReader != nil && cfg.Body != nil {
		return nil, ErrRequestBodyConflict
	}

	if cfg.BodyReader != nil {
		return cfg.BodyReader, nil
	}

	if cfg.Body == nil {
		return nil, nil
	}

	body := cfg.Body

	switch strings.ToLower(contentType) {
	case gomime.CONTENT_TYPE_JSON:
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		return bytes.NewBuffer(payload), nil
	case gomime.CONTENT_TYPE_XML:
		payload, err := xml.Marshal(body)
		if err != nil {
			return nil, err
		}

		return bytes.NewBuffer(payload), nil
	default:
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}

		return bytes.NewBuffer(payload), nil
	}
}

func newRequestConfig(opts ...RequestOption) (*RequestConfig, error) {
	cfg := &RequestConfig{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	if cfg.Headers == nil {
		cfg.Headers = make(http.Header)
	}

	return cfg, nil
}

func (c *Client) readResponseBody(body io.Reader, limit int64) ([]byte, bool, error) {
	if limit <= 0 {
		payload, err := io.ReadAll(body)
		return payload, false, err
	}

	limitedBody := &io.LimitedReader{
		R: body,
		N: limit + 1,
	}

	payload, err := io.ReadAll(limitedBody)
	if err != nil {
		return nil, false, err
	}

	if int64(len(payload)) > limit {
		return payload, true, nil
	}

	return payload, false, nil
}

func (c *Client) getMaxBodyBytes(cfg *RequestConfig) int64 {
	if cfg != nil && cfg.MaxBodyBytes > 0 {
		return cfg.MaxBodyBytes
	}

	if c.builder != nil && c.builder.maxBodyBytes > 0 {
		return c.builder.maxBodyBytes
	}

	return 0
}

func (c *Client) getFinalURL(response *http.Response) string {
	if response == nil || response.Request == nil || response.Request.URL == nil {
		return ""
	}

	return response.Request.URL.String()
}

func (c *Client) getRedirectCount(response *http.Response) int {
	if response == nil || response.Request == nil {
		return 0
	}

	redirects := 0
	for previous := response.Request.Response; previous != nil; {
		redirects++
		if previous.Request == nil {
			break
		}

		previous = previous.Request.Response
	}

	return redirects
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

		transport := c.builder.transport
		if transport == nil {
			transport = &http.Transport{
				MaxIdleConns:          c.getMaxIdleConnections(),
				ResponseHeaderTimeout: c.getResponseTimeout(),
				DialContext: (&net.Dialer{
					Timeout: c.getConnectionTimeout(),
				}).DialContext,
			}
		}

		c.coreClient = &http.Client{
			Timeout:       c.getConnectionTimeout() + c.getResponseTimeout(),
			Transport:     transport,
			CheckRedirect: c.getCheckRedirect(),
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

func (c *Client) getCheckRedirect() func(req *http.Request, via []*http.Request) error {
	if c.builder == nil {
		return nil
	}

	if c.builder.checkRedirect != nil {
		return c.builder.checkRedirect
	}

	if c.builder.redirectPolicy == nil {
		return nil
	}

	policy := *c.builder.redirectPolicy

	return func(req *http.Request, via []*http.Request) error {
		if policy.MaxRedirects > 0 && len(via) > policy.MaxRedirects {
			return http.ErrUseLastResponse
		}

		if policy.Validate != nil {
			return policy.Validate(req, via)
		}

		return nil
	}
}
