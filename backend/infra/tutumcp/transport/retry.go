package transport

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"time"
)

const (
	defaultRetryAttempts = 3
	defaultRetryDelay    = 300 * time.Millisecond
	defaultRetryMaxDelay = 5 * time.Second
	defaultRetryJitter   = 0.2
	unitIntervalMidpoint = 0.5
)

type RetryPolicy struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      float64
}

func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts: defaultRetryAttempts,
		BaseDelay:   defaultRetryDelay,
		MaxDelay:    defaultRetryMaxDelay,
		Jitter:      defaultRetryJitter,
	}
}

func (p RetryPolicy) normalized() RetryPolicy {
	if p.MaxAttempts < 1 {
		p.MaxAttempts = 1
	}

	if p.BaseDelay <= 0 {
		p.BaseDelay = defaultRetryDelay
	}

	if p.MaxDelay < p.BaseDelay {
		p.MaxDelay = p.BaseDelay
	}

	if p.Jitter < 0 || p.Jitter > 1 {
		p.Jitter = defaultRetryJitter
	}

	return p
}

func (p RetryPolicy) delay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter > 0 {
		if retryAfter > p.MaxDelay {
			return p.MaxDelay
		}

		return retryAfter
	}

	delay := p.BaseDelay
	for i := 1; i < attempt; i++ {
		delay *= 2
		if delay >= p.MaxDelay {
			delay = p.MaxDelay

			break
		}
	}

	if p.Jitter > 0 {
		spread := float64(delay) * p.Jitter
		delay = time.Duration(float64(delay) - spread + randomUnit()*2*spread)
	}

	if delay < 0 {
		delay = p.BaseDelay
	}

	return delay
}

func randomUnit() float64 {
	var bytes [8]byte
	if _, err := cryptorand.Read(bytes[:]); err != nil {
		return unitIntervalMidpoint
	}

	return float64(binary.LittleEndian.Uint64(bytes[:])) / float64(math.MaxUint64)
}

func retryable(err error, idempotent bool) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	if errors.Is(err, ErrSessionExpired) {
		return false
	}

	var httpErr *HTTPError
	if errors.As(err, &httpErr) {
		if idempotent {
			return httpErr.Temporary()
		}

		return httpErr.Rejected()
	}

	if !idempotent {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	return errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, net.ErrClosed)
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}

	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryAfterDuration(value string) time.Duration {
	if value == "" {
		return 0
	}

	if seconds, err := time.ParseDuration(value + "s"); err == nil && seconds > 0 {
		return seconds
	}

	if deadline, err := time.Parse(time.RFC1123, value); err == nil {
		if wait := time.Until(deadline); wait > 0 {
			return wait
		}
	}

	return 0
}
