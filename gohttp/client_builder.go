package gohttp

import (
	"net/http"
	"time"
)

type RedirectPolicy struct {
	MaxRedirects int
	Validate     func(req *http.Request, via []*http.Request) error
}

type clientBuilder struct {
	maxIdleConnections int
	connectionTimeout  time.Duration
	responseTimeout    time.Duration
	maxBodyBytes       int64
	transport          http.RoundTripper
	checkRedirect      func(req *http.Request, via []*http.Request) error
	redirectPolicy     *RedirectPolicy
	headers            http.Header
	disabledTimeouts   bool
	client             *http.Client
	userAgent          string
}

type ClientBuilder interface {
	SetHeaders(headers http.Header) ClientBuilder
	SetConnectionTimeout(timeout time.Duration) ClientBuilder
	SetResponseTimeout(timeout time.Duration) ClientBuilder
	SetMaxIdleConnections(maxConnections int) ClientBuilder
	SetMaxBodyBytes(limit int64) ClientBuilder
	SetTransport(transport http.RoundTripper) ClientBuilder
	SetCheckRedirect(fn func(req *http.Request, via []*http.Request) error) ClientBuilder
	SetRedirectPolicy(policy RedirectPolicy) ClientBuilder
	DisableTimeouts(b bool) ClientBuilder
	SetHttpClient(c *http.Client) ClientBuilder
	SetUserAgent(userAgent string) ClientBuilder

	Build() HttpClient
}

func NewBuilder() ClientBuilder {
	builder := &clientBuilder{}

	return builder
}

func (c *clientBuilder) Build() HttpClient {
	builderSnapshot := c.clone()
	client := Client{
		builder: builderSnapshot,
	}

	return &client
}

func (c *clientBuilder) SetHeaders(headers http.Header) ClientBuilder {
	c.headers = headers

	return c
}

func (c *clientBuilder) SetConnectionTimeout(timeout time.Duration) ClientBuilder {
	c.connectionTimeout = timeout

	return c
}

func (c *clientBuilder) SetResponseTimeout(timeout time.Duration) ClientBuilder {
	c.responseTimeout = timeout

	return c
}

func (c *clientBuilder) SetMaxIdleConnections(maxConnections int) ClientBuilder {
	c.maxIdleConnections = maxConnections

	return c
}

func (c *clientBuilder) SetMaxBodyBytes(limit int64) ClientBuilder {
	c.maxBodyBytes = limit

	return c
}

func (c *clientBuilder) SetTransport(transport http.RoundTripper) ClientBuilder {
	c.transport = transport

	return c
}

func (c *clientBuilder) SetCheckRedirect(fn func(req *http.Request, via []*http.Request) error) ClientBuilder {
	c.checkRedirect = fn
	c.redirectPolicy = nil

	return c
}

func (c *clientBuilder) SetRedirectPolicy(policy RedirectPolicy) ClientBuilder {
	c.redirectPolicy = &policy
	c.checkRedirect = nil

	return c
}

func (c *clientBuilder) DisableTimeouts(disabledTimeouts bool) ClientBuilder {
	c.disabledTimeouts = disabledTimeouts

	return c
}

func (c *clientBuilder) SetHttpClient(client *http.Client) ClientBuilder {
	c.client = client

	return c
}

func (c *clientBuilder) SetUserAgent(userAgent string) ClientBuilder {
	c.userAgent = userAgent

	return c
}

func (c *clientBuilder) clone() *clientBuilder {
	if c == nil {
		return &clientBuilder{}
	}

	clone := *c
	clone.headers = cloneHeaders(c.headers)
	if c.redirectPolicy != nil {
		policyCopy := *c.redirectPolicy
		clone.redirectPolicy = &policyCopy
	}

	return &clone
}

func cloneHeaders(headers http.Header) http.Header {
	if headers == nil {
		return nil
	}

	clone := make(http.Header, len(headers))
	for key, values := range headers {
		clone[key] = append([]string(nil), values...)
	}

	return clone
}
