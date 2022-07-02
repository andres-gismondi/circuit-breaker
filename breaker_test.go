package cb_test

import (
	cb "github.com/andres-gismondi/circuit-breaker"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

func TestBreaker_Open(t *testing.T) {
	b := cb.NewBreaker()
	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error {
			return nil
		})
	}

	assert.Equal(t, cb.Open, b.Status)
}

func TestBreaker_Closed(t *testing.T) {
	b := cb.NewBreaker(
		cb.WaitingTime(10),
		cb.Counter(4))

	for i := 0; i < 5; i++ {
		_ = b.Execute(func() error {
			c := http.Client{}
			_, _ = c.Get("http://localhost:8080/test")
			return nil
		})
	}

	assert.Equal(t, cb.Closed, b.Status)
}

func TestBreaker_HalfOpen(t *testing.T) {
	b := cb.NewBreaker(
		cb.WaitingTime(2),
		cb.Counter(1))

	for i := 0; i < 3; i++ {
		_ = b.Execute(func() error {
			c := http.Client{}
			_, _ = c.Get("http://localhost:8080/test")
			return nil
		})
	}

	time.Sleep(3 * time.Second)
	assert.Equal(t, cb.HalfOpen, b.Status)
}
