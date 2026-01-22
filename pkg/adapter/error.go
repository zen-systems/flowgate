package adapter

import (
	"context"
	"errors"
	"fmt"
	"net"
)

// AdapterError wraps provider errors with status metadata.
type AdapterError struct {
	Status    int
	Temporary bool
	Err       error
}

func (e *AdapterError) Error() string {
	if e == nil {
		return "adapter error"
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("adapter error (status=%d)", e.Status)
}

func (e *AdapterError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// IsTransient reports whether an error is safe to retry.
func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var adapterErr *AdapterError
	if errors.As(err, &adapterErr) {
		if adapterErr.Temporary {
			return true
		}
		if adapterErr.Status == 429 || (adapterErr.Status >= 500 && adapterErr.Status <= 599) {
			return true
		}
	}
	return false
}
