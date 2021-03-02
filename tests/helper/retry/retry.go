package retry

import (
	"fmt"
	"testing"
	"time"
)

const (
	// DefaultTimeout the default timeout for the entire retry operation
	DefaultTimeout = time.Second * 30

	// DefaultDelay the default delay between successive retry attempts
	DefaultDelay = time.Millisecond * 10

	// DefaultRetryCount the default retry count between successive retry attempts
	DefaultRetryCount = 6
)

var (
	defaultConfig = Config{
		Timeout:    DefaultTimeout,
		Delay:      DefaultDelay,
		RetryCount: DefaultRetryCount,
	}
)

type Config struct {
	Timeout    time.Duration
	Delay      time.Duration
	RetryCount int8
}

// Option for a retry opteration.
type Option func(cfg *Config)

// Timeout sets the timeout for the entire retry operation.
func Timeout(timeout time.Duration) Option {
	return func(cfg *Config) {
		cfg.Timeout = timeout
	}
}

// Delay sets the delay between successive retry attempts.
func Delay(delay time.Duration) Option {
	return func(cfg *Config) {
		cfg.Delay = delay
	}
}

// RetryCount sets the RetryCount between successive retry attempts.
func Count(count int8) Option {
	return func(cfg *Config) {
		cfg.RetryCount = count
	}
}

// RetriableFunc a function that can be retried.
type RetriableFunc func() (result interface{}, completed bool, err error)

// UntilSuccess retries the given function until success, timeout, or until the passed-in function returns nil.
func UntilSuccess(fn func() error, options ...Option) error {
	_, e := Do(func() (interface{}, bool, error) {
		err := fn()
		if err != nil {
			return nil, false, err
		}

		return nil, true, nil
	}, options...)

	return e
}

// UntilSuccessOrFail calls UntilSuccess, and fails t with Fatalf if it ends up returning an error
func UntilSuccessOrFail(t *testing.T, fn func() error, options ...Option) {
	t.Helper()
	err := UntilSuccess(fn, options...)
	if err != nil {
		t.Fatalf("retry.UntilSuccessOrFail: %v", err)
	}
}

// Do retries the given function, until there is a timeout, or until the function indicates that it has completed.
func Do(fn RetriableFunc, options ...Option) (interface{}, error) {
	cfg := defaultConfig
	var retries int8

	for _, option := range options {
		option(&cfg)
	}

	var lasterr error
	to := time.After(cfg.Timeout)
	for {
		select {
		case <-to:
			return nil, fmt.Errorf("timeout while waiting (last error: %v)", lasterr)
		default:
		}

		if retries > cfg.RetryCount {
			return nil, fmt.Errorf("retried %d times, failing now", cfg.RetryCount)
		}

		retries++
		result, completed, err := fn()
		if completed {
			return result, err
		}
		if err != nil {
			lasterr = err
		}

		<-time.After(cfg.Delay)
	}
}
