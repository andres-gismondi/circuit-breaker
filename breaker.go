package cb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Breaker struct {
	timeout      time.Duration
	counter      int8
	status       Status
	sleepBarrier bool

	ctx   context.Context
	mutex sync.Mutex
	half  half
}

type half struct {
	counter  int8
	sucCount int8
	errCount int8
	reqChan  chan func() error
	resChan  chan error
}

type Status int

const (
	Open Status = iota
	HalfOpen
	Closed

	defaultCounter int8 = 5
)

func NewBreaker(opts ...BreakerOption) *Breaker {
	cb := &Breaker{
		mutex:        sync.Mutex{},
		sleepBarrier: false,
		status:       Open,
		half: half{
			reqChan:  make(chan func() error),
			resChan:  make(chan error),
			sucCount: 6, // TODO: use a percentage in code, not in a variable
			errCount: 5,
			counter:  2,
		},
	}

	defaultOpt := []BreakerOption{
		WaitingTime(5 * time.Second),
		Counter(defaultCounter),
		WithContext(context.Background()),
	}

	options := append(defaultOpt, opts...)
	for _, o := range options {
		o(cb)
	}

	go cb.halfOpen()
	return cb
}

func (cb *Breaker) Execute(req func() error) error {
	switch cb.status {
	case Open:
		return cb.doOpen(req)
	case HalfOpen:
		return cb.doHalfOpen(req)
	case Closed:
		return errors.New("circuit breaker closed. Try later")
	default:
		return nil
	}
}

func (cb *Breaker) doOpen(req func() error) error {
	err := req()
	if err != nil {
		cb.doMutex(func() {
			if cb.counter > 0 {
				cb.counter--
				return
			}
			cb.status = Closed
		})

		if cb.status == Closed {
			go cb.sleep()
		}

		return fmt.Errorf("open error executing request:%w", err)
	}
	return nil
}

func (cb *Breaker) sleep() {
	// Circuit breaker sleep
	cb.mutex.Lock()
	if cb.sleepBarrier {
		cb.mutex.Unlock()
		return
	}
	cb.sleepBarrier = true
	cb.mutex.Unlock()

	<-time.After(cb.timeout)

	cb.doMutex(func() {
		cb.status = HalfOpen
	})
}

func (cb *Breaker) doHalfOpen(req func() error) error {
	cb.half.reqChan <- req
	err := <-cb.half.resChan
	if err != nil {
		return fmt.Errorf("error executing request half open:%w", err)
	}
	return nil
}

func (cb *Breaker) halfOpen() {
	for {
		select {
		case req := <-cb.half.reqChan:
			// Let get in only middle counter requests
			cb.half.counter--
			if cb.half.counter < 0 {
				cb.status = Closed
				cb.half.resChan <- errors.New("error")
			} else {
				cb.executeHalfOpen(req)
			}
		}
	}
}

func (cb *Breaker) executeHalfOpen(req func() error) {
	err := req()
	if err != nil {
		cb.half.errCount--
		if cb.half.errCount < 0 {
			cb.sleepBarrier = false
			go cb.sleep()
		}
		cb.half.resChan <- err
	} else {
		cb.half.sucCount--
		if cb.half.sucCount < 0 {
			cb.doMutex(func() {
				cb.status = Open
				cb.counter = defaultCounter
			})
		}
		cb.half.resChan <- err
	}
}

func (cb *Breaker) doMutex(fn func()) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	fn()
}

func (cb *Breaker) print(from string) {
	cb.doMutex(func() {
		fmt.Printf("%v --> %v", from, cb)
		fmt.Println()
	})
}
