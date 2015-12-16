package api

import (
	"sync"
	"testing"
	"time"

	"github.com/vladimirvivien/automi/testutil"
	"github.com/vladimirvivien/automi/api/tuple"

	"golang.org/x/net/context"
)

func TestOperator_New(t *testing.T) {
	o := NewOperator(context.Background())

	if o.output == nil {
		t.Fatal("Missing output")
	}

	if o.op != nil {
		t.Fatal("Processing element should be nil")
	}

	if o.concurrency != 1 {
		t.Fatal("Concurrency should be initialized to 1.")
	}
}
func TestOperator_Params(t *testing.T) {
	o := NewOperator(context.Background())
	op := OpFunc(func(ctx context.Context, data interface{}) interface{} {
		return nil
	})
	in := make(chan interface{})

	o.SetOperation(op)
	if o.op == nil {
		t.Fatal("process Elem not set")
	}

	o.SetConcurrency(4)
	if o.concurrency != 4 {
		t.Fatal("Concurrency not being set")
	}

	o.SetInput(in)
	if o.input == nil {
		t.Fatal("Input not being set")
	}

	if o.GetOutput == nil {
		t.Fatal("Output not set")
	}
}

func TestOperator_Exec(t *testing.T) {
	ctx, _ := context.WithCancel(context.Background())
	o := NewOperator(ctx)

	op := OpFunc(func(ctx context.Context, data interface{}) interface{} {
		values := data.(tuple.Tuple).Values()
		t.Logf("Processing data %v, sending %d", values, len(values))
		return len(values)
	})
	o.SetOperation(op)

	in := make(chan interface{})
	go func() {
		in <- tuple.New("A", "B", "C")
		in <- tuple.New("D", "E")
		in <- tuple.New("G")
		close(in)
	}()
	o.SetInput(in)

	wait := make(chan struct{})
	go func() {
		defer close(wait)
		for data := range o.GetOutput() {
			val, ok := data.(int)
			t.Logf("Got value %v", val)
			if !ok {
				t.Fatalf("Expecting type int, got %T, value %v", val, val)
			}
			if val != 3 && val != 2 && val != 1 {
				t.Fatalf("Expecting values 3, 2, or 1, but got %d", val)
			}
		}
	}()

	if err := o.Exec(context.Background()); err != nil {
		t.Fatal(err)
	}

	select {
	case <-wait:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Took too long...")
	}
}

func BenchmarkOperator_Exec(b *testing.B) {
	ctx := context.Background()
	o := NewOperator(ctx)
	N := b.N

	chanSize := func() int {
		if N == 1 {
			return N
		}
		return int(float64(0.5) * float64(N))
	}()

	in := make(chan interface{}, chanSize)
	o.SetInput(in)
	go func() {
		for i := 0; i < N; i++ {
			in <- tuple.New(testutil.GenWord())
		}
		close(in)
	}()

	counter := 0
	var m sync.RWMutex

	op := OpFunc(func(ctx context.Context, data interface{}) interface{} {
		m.Lock()
		counter++
		m.Unlock()
		return data
	})
	o.SetOperation(op)

	// process output
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _ = range o.GetOutput() {
		}
	}()

	if err := o.Exec(ctx); err != nil {
		b.Fatal("Error during execution:", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second * 60):
		b.Fatal("Took too long")
	}
	m.RLock()
	b.Logf("Input %d, counted %d", N, counter)
	if counter != N {
		b.Fatalf("Expected %d items processed,  got %d", N, counter)
	}
	m.RUnlock()
}
