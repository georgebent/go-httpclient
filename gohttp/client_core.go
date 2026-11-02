package gohttp

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/georgebent/go-httpclient/core"
	"github.com/georgebent/go-httpclient/gomime"
)

const (
	DEFAULT_MAX_IDDLE_CONNECTIONS = 5
	DEFAULT_RESPONSE_TIMEOUT      = 5 * time.Second
	DEFAULT_CONNECTION_TIMEOUT    = 1 * time.Second
	DEFAULT_MAX_REDIRECTS         = 10
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
	requestBody, err := c.getRequestBody(fullHeaders, requestConfig)
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

	copyHeaders(result, c.builder.headers)
	copyHeaders(result, requestHeaders)

	if c.builder.userAgent != "" && result.Get(gomime.HEADER_USER_AGENT) == "" {
		result.Set(gomime.HEADER_USER_AGENT, c.builder.userAgent)
	}

	return result
}

func copyHeaders(dst, src http.Header) {
	for header, values := range src {
		if len(values) == 0 {
			continue
		}

		dst[http.CanonicalHeaderKey(header)] = append([]string(nil), values...)
	}
}

func (c *Client) getRequestBody(headers http.Header, cfg *RequestConfig) (io.Reader, error) {
	if cfg.BodyReader != nil && cfg.Body != nil {
		return nil, ErrRequestBodyConflict
	}

	if cfg.BodyReader != nil {
		return cfg.BodyReader, nil
	}

	switch body := cfg.Body.(type) {
	case nil:
		return nil, nil
	case []byte:
		return bytes.NewReader(body), nil
	case string:
		return strings.NewReader(body), nil
	}

	contentType := headers.Get(gomime.HEADER_CONTENT_TYPE)

	if isXMLContentType(contentType) {
		payload, err := xml.Marshal(cfg.Body)
		if err != nil {
			return nil, err
		}

		return bytes.NewReader(payload), nil
	}

	payload, err := json.Marshal(cfg.Body)
	if err != nil {
		return nil, err
	}

	if contentType == "" {
		headers.Set(gomime.HEADER_CONTENT_TYPE, gomime.CONTENT_TYPE_JSON)
	}

	return bytes.NewReader(payload), nil
}

func isXMLContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType, _, _ = strings.Cut(contentType, ";")
		mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	}

	return mediaType == gomime.CONTENT_TYPE_XML || mediaType == "text/xml" || strings.HasSuffix(mediaType, "+xml")
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

func (c *Client) getHttpClient() *http.Client {
	c.clientOnce.Do(func() {
		if c.builder.client != nil {
			c.coreClient = c.builder.client
			return
		}

		transport := c.builder.transport
		if transport == nil {
			transport = c.newTransport()
		}

		c.coreClient = &http.Client{
			Timeout:       c.builder.requestTimeout,
			Transport:     transport,
			CheckRedirect: c.getCheckRedirect(),
		}
	})

	return c.coreClient
}

func (c *Client) newTransport() *http.Transport {
	transport, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		transport = transport.Clone()
	} else {
		transport = &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			ForceAttemptHTTP2:     true,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
	}

	transport.MaxIdleConns = c.getMaxIdleConnections()
	transport.MaxIdleConnsPerHost = c.getMaxIdleConnections()
	transport.ResponseHeaderTimeout = c.getResponseTimeout()
	transport.DialContext = (&net.Dialer{
		Timeout:   c.getConnectionTimeout(),
		KeepAlive: 30 * time.Second,
	}).DialContext

	return transport
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
	maxRedirects := policy.MaxRedirects
	if maxRedirects <= 0 {
		maxRedirects = DEFAULT_MAX_REDIRECTS
	}

	return func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return http.ErrUseLastResponse
		}

		if policy.Validate != nil {
			return policy.Validate(req, via)
		}

		return nil
	}
}
