package gohttp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
)

type ErrorKind string

const (
	ErrorKindUnknown    ErrorKind = "unknown"
	ErrorKindTimeout    ErrorKind = "timeout"
	ErrorKindDNS        ErrorKind = "dns"
	ErrorKindTLS        ErrorKind = "tls"
	ErrorKindConnection ErrorKind = "connection"
	ErrorKindCanceled   ErrorKind = "canceled"
)

func IsTimeout(err error) bool {
	var netErr net.Error

	return errors.As(err, &netErr) && netErr.Timeout()
}

func IsDNS(err error) bool {
	var dnsErr *net.DNSError

	return errors.As(err, &dnsErr)
}

func IsTLS(err error) bool {
	var recordHeaderErr tls.RecordHeaderError
	var certificateVerificationErr *tls.CertificateVerificationError

	return errors.As(err, &recordHeaderErr) || errors.As(err, &certificateVerificationErr)
}

func IsConnection(err error) bool {
	var opErr *net.OpError

	return errors.As(err, &opErr) && !IsDNS(err) && !IsTimeout(err)
}

func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func ClassifyError(err error) ErrorKind {
	switch {
	case err == nil:
		return ErrorKindUnknown
	case IsCanceled(err):
		return ErrorKindCanceled
	case IsTimeout(err):
		return ErrorKindTimeout
	case IsDNS(err):
		return ErrorKindDNS
	case IsTLS(err):
		return ErrorKindTLS
	case IsConnection(err):
		return ErrorKindConnection
	default:
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return ClassifyError(urlErr.Err)
		}

		return ErrorKindUnknown
	}
}

func IsRedirectError(err error) bool {
	return errors.Is(err, http.ErrUseLastResponse)
}
