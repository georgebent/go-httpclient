package core

import (
	"encoding/json"
	"net/http"
	"time"
)

type Response struct {
	Status        string
	StatusCode    int
	Headers       http.Header
	Body          []byte
	FinalURL      string
	ContentLength int64
	Duration      time.Duration
	RedirectCount int
}

func (r *Response) BodyBytes() []byte {
	return r.Body
}

func (r *Response) BodyString() string {
	return string(r.Body)
}

func (r *Response) UnmarshalJson(target interface{}) error {
	return json.Unmarshal(r.BodyBytes(), target)
}
