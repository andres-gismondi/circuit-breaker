package cb

import (
	"context"
	"time"
)

type BreakerOption func(b *Breaker)

// WaitingTime set timer before test again the net
func WaitingTime(time time.Duration) BreakerOption {
	return func(b *Breaker) {
		b.timeout = time
	}
}

// Counter set quantity of retries before stop requests
func Counter(count int8) BreakerOption {
	return func(b *Breaker) {
		b.counter = count
	}
}

func WithContext(ctx context.Context) BreakerOption {
	return func(b *Breaker) {
		b.ctx = ctx
	}
}
