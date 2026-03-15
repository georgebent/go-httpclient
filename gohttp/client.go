package gohttp

import (
	"net/http"
	"sync"

	"github.com/georgebent/go-httpclient/core"
)

type HttpClient interface {
	Get(url string, headers http.Header) (*core.Response, error)
	Post(url string, headers http.Header, body interface{}) (*core.Response, error)
	Put(url string, headers http.Header, body interface{}) (*core.Response, error)
	Delete(url string, headers http.Header) (*core.Response, error)
	Patch(url string, headers http.Header, body interface{}) (*core.Response, error)
}

type Client struct {
	coreClient *http.Client
	builder    *clientBuilder
	clientOnce sync.Once
}

func (c *Client) Get(url string, headers http.Header) (*core.Response, error) {
	return c.do(http.MethodGet, url, headers, nil)
}

func (c *Client) Post(url string, headers http.Header, body interface{}) (*core.Response, error) {
	return c.do(http.MethodPost, url, headers, body)
}

func (c *Client) Put(url string, headers http.Header, body interface{}) (*core.Response, error) {
	return c.do(http.MethodPut, url, headers, body)
}

func (c *Client) Delete(url string, headers http.Header) (*core.Response, error) {
	return c.do(http.MethodDelete, url, headers, nil)
}
func (c *Client) Patch(url string, headers http.Header, body interface{}) (*core.Response, error) {
	return c.do(http.MethodPatch, url, headers, body)
}
