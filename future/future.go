package future

import (
	"context"
	"fmt"
	"sync/atomic"
)

type Future[T any] interface {
	Await(context.Context) (T, error)
}

type future[T any] struct {
	doneCh   chan struct{}
	resultCh chan T
	errCh    chan error
}

func newFuture[T any]() *future[T] {
	return &future[T]{
		doneCh:   make(chan struct{}),
		resultCh: make(chan T),
		errCh:    make(chan error),
	}
}

func (f *future[T]) setResult(result T) {
	select {
	case f.resultCh <- result:
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
		close(f.doneCh)
	}()
	select {
	case <-ctx.Done():
		return t, ctx.Err()
	case result := <-f.resultCh:
		return result, nil
	case err := <-f.errCh:
		return t, err
	}
}

func All[T any](ctx context.Context, futures ...Future[T]) Future[[]T] {
	f := newFuture[[]T]()

	results := make([]T, len(futures))
	resultCnt := int64(0)

	for i, future := range futures {
		go func(i int, future Future[T]) {
			result, err := future.Await(ctx)
			if err != nil {
				f.setError(err)
				return
			}
			results[i] = result
			if atomic.AddInt64(&resultCnt, 1) == int64(len(futures)) {
				f.setResult(results)
			}
		}(i, future)
	}

	return f
}

func Any[T any](ctx context.Context, futures ...Future[T]) Future[T] {
	f := newFuture[T]()

	var firstErr error
	resultCnt := int64(0)

	for i, future := range futures {
		go func(i int, future Future[T]) {
			result, err := future.Await(ctx)
			if err != nil {
				if firstErr == nil {
					firstErr = err
				}
				if atomic.AddInt64(&resultCnt, 1) == int64(len(futures)) {
					f.setError(firstErr)
				}
			} else {
				f.setResult(result)
			}
		}(i, future)
	}

	return f
}

func Race[T any](ctx context.Context, futures ...Future[T]) Future[T] {
	f := newFuture[T]()

	for i, future := range futures {
		go func(i int, future Future[T]) {
			result, err := future.Await(ctx)
			if err != nil {
				f.setError(err)
				return
			}
			f.setResult(result)
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
		result, err := fnc()
		if err != nil {
			f.setError(err)
			return
		}
		f.setResult(result)
	}()

	return f
}
