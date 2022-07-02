package cb_test

import (
	"fmt"
	cb "github.com/andres-gismondi/circuit-breaker"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
	"time"
)

func TestBreaker_Execute(t *testing.T) {
	breaker := cb.NewBreaker(
		cb.WaitingTime(3*time.Second),
		cb.Counter(4))
	for i := 0; i < 5; i++ {
		go testbreaker(breaker, t)
	}
	time.Sleep(1 * time.Second)
	for j := 0; j < 5; j++ {
		go testbreaker(breaker, t)
	}

	time.Sleep(1 * time.Second)
	for i := 0; i < 5; i++ {
		go testbreaker(breaker, t)
	}

	time.Sleep(1 * time.Second)
	for i := 0; i < 5; i++ {
		go testbreaker(breaker, t)
	}

	time.Sleep(5 * time.Second)
}

func testbreaker(breaker *cb.Breaker, t *testing.T) {
	err := breaker.Execute(func() error {
		c := http.Client{}
		_, err := c.Get("http://localhost:9290/hola")
		time.Sleep(1 * time.Second)
		if err != nil {
			return err
		}
		return nil
	})
	assert.Nil(t, err)
}

type pp struct {
	value int
}

func (p *pp) change() {
	p.value = 10
	time.Sleep(3 * time.Second)
	fmt.Println(p.value)
}
func TestPointer(t *testing.T) {
	t.Skip()
	sp := &pp{value: 5}
	go sp.change()
	time.Sleep(1 * time.Second)
	fmt.Println(sp.value)
	time.Sleep(6 * time.Second)
}
