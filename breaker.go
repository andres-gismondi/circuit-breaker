package cb

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Breaker struct {
	timeout time.Duration
	counter int8
	Status  Status
	window  time.Duration

	errPercentage float64
	ctx           context.Context
	mutex         sync.Mutex
	wake          chan bool

	wt      windowTime
	half    half
	mutable state
}

type half struct {
	counter  int8
	sucCount int8
	errCount int8
	reqChan  chan func() error
	resChan  chan error
}

type state struct {
	counter      int8
	halfCounter  int8
	errCounter   int8
	sucCounter   int8
	sleepBarrier bool
}

type windowTime struct {
	first time.Time
}

type Status int

const (
	Open Status = iota
	HalfOpen
	Closed
)

var (
	ErrExecuted = errors.New("service error")
	ErrHalfOpen = errors.New("half open excluded")
	ErrClosed   = errors.New("closed circuit breaker")
)

func NewBreaker(opts ...BreakerOption) *Breaker {

	cb := &Breaker{
		mutex:  sync.Mutex{},
		Status: Open,
		wt:     windowTime{},
		half: half{
			reqChan: make(chan func() error),
			resChan: make(chan error),
		},
	}

	defaultOpt := []BreakerOption{
		WaitingTime(5 * time.Second),
		Counter(5),
		WithContext(context.Background()),
		ErrorPercentage(100),
		WindowTime(5 * time.Second),
	}

	options := append(defaultOpt, opts...)
	for _, o := range options {
		o(cb)
	}

	errP := int8(float64(cb.counter) * cb.errPercentage)
	if errP == cb.counter {
		errP = cb.counter - 1
	}
	cb.half.errCount = errP
	cb.half.sucCount = cb.counter - errP
	cb.half.counter = cb.counter

	cb.mutable = state{
		sleepBarrier: false,
		counter:      cb.counter,
		halfCounter:  cb.half.counter,
		errCounter:   cb.half.errCount,
		sucCounter:   cb.half.sucCount,
	}

	go cb.halfOpen()
	go cb.wakeUpBreaker()
	return cb
}

func (cb *Breaker) Execute(req func() error) error {
	switch cb.Status {
	case Open:
		return cb.doOpen(req)
	case HalfOpen:
		return cb.doHalfOpen(req)
	case Closed:
		return ErrClosed
	default:
		return nil
	}
}

func (cb *Breaker) doOpen(req func() error) error {
	err := req()
	if err != nil {
		cb.doMutex(func() {
			if cb.mutable.counter > 0 {
				cb.mutable.counter--

				// first error
				if cb.mutable.counter+1 == cb.counter {
					cb.wt.first = time.Now()
				}
				return
			}

			// verify if duration between first error and last one is inside window
			w := time.Now().Sub(cb.wt.first)
			if w <= cb.window {
				cb.Status = Closed
			} else {
				cb.openState()
			}
		})

		if cb.Status == Closed {
			go cb.sleep()
		}

		return fmt.Errorf("open error executing request:%w", err)
	}
	return nil
}

func (cb *Breaker) sleep() {
	// Circuit breaker sleep
	cb.mutex.Lock()
	if cb.mutable.sleepBarrier {
		cb.mutex.Unlock()
		return
	}
	cb.mutable.sleepBarrier = true
	cb.mutex.Unlock()

	<-time.After(cb.timeout)

	cb.doMutex(func() {
		cb.Status = HalfOpen
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
			cb.mutable.halfCounter--
			if cb.mutable.halfCounter < 0 {
				cb.Status = Closed
				cb.half.resChan <- ErrHalfOpen
			} else {
				cb.executeHalfOpen(req)
			}
		case <-cb.wake:
			cb.doMutex(func() {
				cb.openState()
			})
		}
	}
}

func (cb *Breaker) executeHalfOpen(req func() error) {
	err := req()
	if err != nil {
		cb.mutable.errCounter--
		if cb.mutable.errCounter < 0 {
			cb.mutable.sleepBarrier = false
			go cb.sleep()
		}
		cb.half.resChan <- err
	} else {
		cb.mutable.sucCounter--
		if cb.mutable.sucCounter < 0 {
			cb.doMutex(func() {
				cb.openState()
			})
		}
		cb.half.resChan <- err
	}
}

func (cb *Breaker) wakeUpBreaker() {
	for {
		select {
		case <-time.After(60 * time.Second): //TODO: change this default time
			if cb.Status == Closed {
				cb.wake <- true
			}
		}
	}
}

func (cb *Breaker) openState() {
	cb.Status = Open
	cb.wt = windowTime{}
	cb.mutable = state{
		sleepBarrier: false,
		counter:      cb.counter,
		halfCounter:  cb.half.counter,
		errCounter:   cb.half.errCount,
		sucCounter:   cb.half.sucCount,
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
