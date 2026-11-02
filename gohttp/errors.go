package gohttp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
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
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	netErr, ok := errors.AsType[net.Error](err)

	return ok && netErr.Timeout()
}

func IsDNS(err error) bool {
	_, ok := errors.AsType[*net.DNSError](err)

	return ok
}

func IsTLS(err error) bool {
	if _, ok := errors.AsType[tls.RecordHeaderError](err); ok {
		return true
	}

	if _, ok := errors.AsType[tls.AlertError](err); ok {
		return true
	}

	if opErr, ok := errors.AsType[*net.OpError](err); ok && (opErr.Op == "remote error" || opErr.Op == "local error") {
		return true
	}

	_, ok := errors.AsType[*tls.CertificateVerificationError](err)

	return ok
}

func IsConnection(err error) bool {
	_, ok := errors.AsType[*net.OpError](err)

	return ok && !IsDNS(err) && !IsTimeout(err) && !IsTLS(err)
}

func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled)
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
		return ErrorKindUnknown
	}
}

func IsRedirectError(err error) bool {
	return errors.Is(err, ErrTooManyRedirects)
}
