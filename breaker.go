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

	ctx   context.Context
	mutex sync.Mutex
	enter bool
	test  bool
	midd  middleEnter
}

type middleEnter struct {
	enter     bool
	counter   int8
	succCount int8
	errCount  int8
	testChan  chan func() error
	resChan   chan error
}

const defaultCounter int8 = 5

func NewBreaker(opts ...BreakerOption) *Breaker {
	cb := &Breaker{
		enter: true,
		mutex: sync.Mutex{},
		midd: middleEnter{
			testChan:  make(chan func() error),
			resChan:   make(chan error),
			succCount: 6, // TODO: use a percentage in code, not in a variable
			errCount:  5,
			counter:   defaultCounter,
			enter:     true,
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

	go cb.doTest()
	return cb
}

func (cb *Breaker) Execute(req func() error) error {
	//fmt.Println(cb)
	if !cb.enter {
		return errors.New("circuit breaker activated. Try later")
	}

	if cb.test {

		if !cb.midd.enter {
			return errors.New("dotest circuit breaker activated. Try later")
		}

		cb.midd.testChan <- req
		err := <-cb.midd.resChan
		if err != nil {
			return fmt.Errorf("dotest error executing request:%w", err)
		}
		return nil
	}

	err := req()
	if err != nil {
		go cb.do()
		return fmt.Errorf("do error executing request:%w", err)
	}
	return nil
}

func (cb *Breaker) do() {
	cb.mutex.Lock()
	if cb.counter > 0 || !cb.enter {
		cb.counter--
		cb.mutex.Unlock()
		return
	}

	cb.enter = false
	cb.mutex.Unlock()

	<-time.After(cb.timeout)

	cb.mutex.Lock()
	cb.test = true
	cb.enter = true
	cb.mutex.Unlock()
}

func (cb *Breaker) doTest() {
	req := <-cb.midd.testChan

	cb.midd.counter--
	if cb.midd.counter < 0 {
		cb.midd.enter = false
		cb.midd.resChan <- errors.New("error")
		return
	}

	err := req()
	if err != nil {
		cb.midd.errCount--
		if cb.midd.errCount < 0 {
			cb.freeAccess(0)
		}
		cb.midd.resChan <- err
		return
	}

	cb.midd.succCount--
	if cb.midd.succCount < 0 {
		cb.freeAccess(defaultCounter)
	}
	cb.midd.resChan <- err
}

func (cb *Breaker) freeAccess(count int8) {
	cb.mutex.Lock()
	cb.enter = true
	cb.midd.enter = true
	cb.test = true
	cb.counter = count
	cb.mutex.Unlock()
}

func (cb *Breaker) print(from string) {
	cb.mutex.Lock()
	fmt.Printf("%v --> %v", from, cb)
	fmt.Println()
	cb.mutex.Unlock()
}
