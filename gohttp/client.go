package gohttp

import (
	"context"
	"io"
	"net/http"
	"sync"

	"github.com/georgebent/go-httpclient/core"
)

type HttpClient interface {
	Do(ctx context.Context, method, url string, opts ...RequestOption) (*core.Response, error)
	Get(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error)
	Post(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error)
	Put(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error)
	Delete(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error)
	Patch(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error)
}

type Client struct {
	coreClient *http.Client
	builder    *clientBuilder
	clientOnce sync.Once
}

func (c *Client) Do(ctx context.Context, method, url string, opts ...RequestOption) (*core.Response, error) {
	return c.do(ctx, method, url, opts...)
}

func (c *Client) Get(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error) {
	return c.Do(ctx, http.MethodGet, url, opts...)
}

func (c *Client) Post(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error) {
	return c.Do(ctx, http.MethodPost, url, opts...)
}

func (c *Client) Put(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error) {
	return c.Do(ctx, http.MethodPut, url, opts...)
}

func (c *Client) Delete(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error) {
	return c.Do(ctx, http.MethodDelete, url, opts...)
}

func (c *Client) Patch(ctx context.Context, url string, opts ...RequestOption) (*core.Response, error) {
	return c.Do(ctx, http.MethodPatch, url, opts...)
}

type RequestOption func(*RequestConfig) error

type RequestConfig struct {
	Headers      http.Header
	Body         any
	BodyReader   io.Reader
	MaxBodyBytes int64
}

func WithHeaders(headers http.Header) RequestOption {
	return func(cfg *RequestConfig) error {
		if headers == nil {
			return nil
		}

		cfg.Headers = cloneHeaders(headers)

		return nil
	}
}

func WithBody(body any) RequestOption {
	return func(cfg *RequestConfig) error {
		cfg.Body = body

		return nil
	}
}

func WithBodyReader(reader io.Reader) RequestOption {
	return func(cfg *RequestConfig) error {
		cfg.BodyReader = reader

		return nil
	}
}

func WithMaxBodyBytes(limit int64) RequestOption {
	return func(cfg *RequestConfig) error {
		cfg.MaxBodyBytes = limit

		return nil
	}
}
