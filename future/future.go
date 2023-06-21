package future

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

type Future[T any] interface {
	Await(context.Context) (T, error)
}

type Result[T any] struct {
	Value T
	Error error
}

type future[T any] struct {
	doneCh  chan struct{}
	valueCh chan T
	errCh   chan error
	closed  bool
}

func newFuture[T any]() *future[T] {
	return &future[T]{
		doneCh:  make(chan struct{}),
		valueCh: make(chan T),
		errCh:   make(chan error),
	}
}

func (f *future[T]) setValue(result T) {
	select {
	case f.valueCh <- result:
	case <-f.doneCh:
	}
}

func (f *future[T]) setError(err error) {
	select {
	case f.errCh <- err:
	case <-f.doneCh:
	}
}

func (f *future[T]) Await(ctx context.Context) (t T, err error) {
	defer func() {
		if !f.closed {
			close(f.doneCh)
			f.closed = true
		}
	}()
	select {
	case <-f.doneCh:
		return t, errors.New("already finished future")
	case <-ctx.Done():
		return t, ctx.Err()
	case result := <-f.valueCh:
		return result, nil
	case err := <-f.errCh:
		return t, err
	}
}

func All[T any](ctx context.Context, futures ...Future[T]) Future[[]T] {
	f := newFuture[[]T]()

	values := make([]T, len(futures))
	valueCnt := int64(0)

	for i, future := range futures {
		go func(i int, future Future[T]) {
			value, err := future.Await(ctx)
			if err != nil {
				f.setError(err)
				return
			}
			values[i] = value
			if atomic.AddInt64(&valueCnt, 1) == int64(len(futures)) {
				f.setValue(values)
			}
		}(i, future)
	}

	return f
}

func AllSettled[T any](ctx context.Context, futures ...Future[T]) Future[[]Result[T]] {
	f := newFuture[[]Result[T]]()

	results := make([]Result[T], len(futures))
	resultCnt := int64(0)

	for i, future := range futures {
		go func(i int, future Future[T]) {
			value, err := future.Await(ctx)
			if err != nil {
				results[i] = Result[T]{
					Error: err,
				}
			} else {
				results[i] = Result[T]{
					Value: value,
				}
			}
			if atomic.AddInt64(&resultCnt, 1) == int64(len(futures)) {
				f.setValue(results)
			}
		}(i, future)
	}

	return f
}

func Any[T any](ctx context.Context, futures ...Future[T]) Future[T] {
	f := newFuture[T]()

	var firstErr error
	errCnt := int64(0)

	for i, future := range futures {
		go func(i int, future Future[T]) {
			value, err := future.Await(ctx)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				if atomic.AddInt64(&errCnt, 1) == int64(len(futures)) {
					f.setError(firstErr)
				}
			} else {
				f.setValue(value)
			}
		}(i, future)
	}

	return f
}

func Race[T any](ctx context.Context, futures ...Future[T]) Future[T] {
	f := newFuture[T]()

	for i, future := range futures {
		go func(i int, future Future[T]) {
			value, err := future.Await(ctx)
			if err != nil {
				f.setError(err)
				return
			}
			f.setValue(value)
		}(i, future)
	}

	return f
}

func New[T any](fnc func() (T, error)) Future[T] {
	f := newFuture[T]()

	go func() {
		defer func() {
			if val := recover(); val != nil {
				if err, ok := val.(error); ok {
					f.setError(err)
				} else {
					f.setError(fmt.Errorf("%v", val))
				}
			}
		}()
		value, err := fnc()
		if err != nil {
			f.setError(err)
			return
		}
		f.setValue(value)
	}()

	return f
}
