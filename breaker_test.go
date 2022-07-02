//go:build !race
// +build !race

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
			_, err := c.Get("http://localhost:8080/test")
			return err
		})
	}

	assert.Equal(t, cb.Closed, b.Status)
}

func TestBreaker_HalfOpen(t *testing.T) {
	b := cb.NewBreaker(
		cb.WaitingTime(2),
		cb.Counter(2))

	for i := 0; i < 5; i++ {
		_ = b.Execute(func() error {
			c := http.Client{}
			_, err := c.Get("http://localhost:8080/test")
			time.Sleep(1 * time.Second)
			return err
		})
	}

	time.Sleep(5 * time.Second)
	assert.Equal(t, cb.HalfOpen, b.Status)
}

/*
func TestBreaker_HalfOpenToClosedState(t *testing.T) {
	b := cb.NewBreaker(
		cb.WaitingTime(1),
		cb.Counter(1))

	for i := 0; i < 10; i++ {
		_ = b.Execute(func() error {
			c := http.Client{}
			_, err := c.Get("http://localhost:8080/test")
			time.Sleep(1 * time.Second)
			return err
		})
	}

	time.Sleep(5 * time.Second)
	assert.Equal(t, cb.Closed, b.Status)
}*/
