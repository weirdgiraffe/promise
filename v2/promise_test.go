package promise

import (
	"errors"
	"testing"
	"time"
)

func TestPromiseIsNotDoneAfterCreation(t *testing.T) {
	p := NewPromise[string]()

	select {
	case <-p.Done():
		t.Fatalf("promise is done before being resolved or rejected")
	default:
	}
}

func TestPromiseCouldBeResolved(t *testing.T) {
	expected := "hello world"

	p := NewPromise[string]()
	p.Resolve(expected)

	select {
	case <-p.Done():
		actual, err := p.Result()
		if err != nil {
			t.Fatalf("expected nil, got error %[1]v (%[1]T)", err)
		}
		if actual != expected {
			t.Logf("exp: %s", expected)
			t.Logf("got: %s", actual)
			t.Fatalf("unexpected result")
		}
	default:
		t.Fatalf("promise is not done after being resolved")
	}
}

func TestPromiseCouldBeRejected(t *testing.T) {
	expectedError := errors.New("hello world")

	p := NewPromise[string]()
	p.Reject(expectedError)

	select {
	case <-p.Done():
		actual, err := p.Result()
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if actual != "" {
			t.Fatalf("unexpected promise result: %v, expected empty string", actual)
		}
		if !errors.Is(err, expectedError) {
			t.Logf("exp: %v", expectedError)
			t.Logf("got: %v", err)
			t.Fatalf("unexpected error")
		}
	default:
		t.Fatalf("promise is not done after being rejected")
	}
}

func TestPromiseCancelation(t *testing.T) {
	p := NewPromise[string]()

	if p.Canceled() {
		t.Fatalf("promise is canceled after creation")
	}

	p.Cancel()

	if !p.Canceled() {
		t.Fatalf("promise was not canceled")
	}
}

func TestPromiseDoubleResolve(t *testing.T) {
	expectedValue := "hello alice"
	unexpectedValue := "hello bob"

	p := NewPromise[string]()
	p.Resolve(expectedValue)
	p.Resolve(unexpectedValue)

	actual, err := p.Result()
	if err != nil {
		t.Fatalf("unexpected error %[1]v (%[1]T)", err)
	}

	if expectedValue != actual {
		t.Logf("exp: %s", expectedValue)
		t.Logf("got: %s", actual)
		t.Errorf("unexpected value")
	}
}

func TestPromiseDoubleReject(t *testing.T) {
	expectedError := errors.New("hello world")
	unexpectedError := errors.New("unexpected error")

	p := NewPromise[string]()
	p.Reject(expectedError)
	p.Reject(unexpectedError)

	_, err := p.Result()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, expectedError) {
		t.Logf("exp: %v", expectedError)
		t.Logf("got: %v", err)
		t.Errorf("unexpected error")
	}
}

func TestPromiseRejectAll(t *testing.T) {
	expectedError := errors.New("hello world")

	l := make([]*Promise[string], 5)
	for i := range l {
		l[i] = NewPromise[string]()
	}

	RejectAll(expectedError, l)

	for i := range l {
		_, err := l[i].Result()
		if !errors.Is(err, expectedError) {
			t.Logf("exp: %[1]T %[1]v", expectedError)
			t.Logf("got: %[1]T %[1]v", err)
			t.Error("unexpected error of rejected promise")
		}
	}
}

func TestRunnerResolvesPromises(t *testing.T) {
	r := NewRunner(DefaultRunnerConcurrency, DefaultRunnerCapacity)
	t.Cleanup(r.Wait)

	expected := "hello world"
	p := Async(r, func() (string, error) {
		return expected, nil
	})

	actual, err := p.Result()
	if err != nil {
		t.Fatalf("unexpected error %[1]v (%[1]T)", err)
	}

	if expected != actual {
		t.Logf("exp: %s", expected)
		t.Logf("got: %s", actual)
		t.Error("unexpected promise value")
	}
}

func TestRunnerRejectsPromises(t *testing.T) {
	r := NewRunner(DefaultRunnerConcurrency, DefaultRunnerCapacity)
	t.Cleanup(r.Wait)

	expected := errors.New("hello world")
	p := Async(r, func() (string, error) {
		return "", expected
	})

	_, err := p.Result()
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, expected) {
		t.Logf("exp: %v", expected)
		t.Logf("got: %v", err)
		t.Error("unexpected promise error kind")
	}
}

func TestRunnerCouldBeWaitedMultipleTimes(t *testing.T) {
	r := NewRunner(1, DefaultRunnerCapacity)

	waited1 := make(chan struct{})
	go func() {
		r.Wait()
		close(waited1)
	}()

	select {
	case <-waited1:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("first wait took more then 100ms")
	}

	waited2 := make(chan struct{})
	go func() {
		r.Wait()
		close(waited2)
	}()

	select {
	case <-waited2:
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("second wait took more then 100ms")
	}
}

func TestRunnerExecutesAllPendingPromises(t *testing.T) {
	r := NewRunner(1, DefaultRunnerCapacity)

	// make some promise for long runnig function
	_ = Async(r, func() (string, error) {
		time.Sleep(100 * time.Millisecond)
		return "", nil
	})

	expected := "hello world"
	p := Async(r, func() (string, error) {
		return expected, nil
	})
	r.Wait()

	actual, err := p.Result()
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if expected != actual {
		t.Logf("exp: %s", expected)
		t.Logf("got: %s", actual)
		t.Error("unexpected promise value")
	}

	shouldReject := Async(r, func() (string, error) {
		return "", nil
	})
	_, err = shouldReject.Result()
	if err == nil {
		t.Fatalf("promise on waited runner shold error, but it is not")
	}
	if !errors.Is(err, ErrExecutionDone) {
		t.Logf("exp: %v", ErrExecutionDone)
		t.Logf("got: %v", err)
		t.Error("unexpected error for promise on waited runner")
	}

}
