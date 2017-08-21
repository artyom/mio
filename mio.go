// Package mio provides tools to combine io.Writer and
// metrics.Histogram interfaces, so that every non-empty Write call latency
// value is sampled to Histogram.
//
// Writer and Reader can be used to bind standard metrics.Histogram to io.Writer
// and io.Reader.  SelfCleaningHistogram provides a wrapper over
// metrics.Histogram with self-cleaning capabilities, which can be used for
// sharing one Histogram over multiple writers/readers and cleaning sample pool
// after period of inactivity.
//
// This package is intended to be used with go-metrics:
// (https://github.com/rcrowley/go-metrics) or metrics
// (https://github.com/facebookgo/metrics) packages.
package mio

import (
	"context"
	"time"
)

// Histogram interface wraps a subset of methods of metrics.Histogram interface
// so it can be used without type conversion.
type Histogram interface {
	Clear()
	Count() int64
	Max() int64
	Mean() float64
	Min() int64
	Percentile(float64) float64
	Percentiles([]float64) []float64
	StdDev() float64
	Update(int64)
	Variance() float64
}

// SelfCleaningHistogram wraps metrics.Histogram, adding self-cleaning feature
// if no samples were registered for a specified time. SelfCleaningHistogram
// also implements Registrar interface, call Register() method to announce
// following sample updates, call Done() after all samples were added. If no
// outstanding workers registered (for each Register() call Done() call were
// made), self-cleaning timer would start, cleaning histogram's sample pool in
// absence of Register() calls before timer fires.
type SelfCleaningHistogram struct {
	Histogram
	c      chan int
	ctx    context.Context
	cancel func()
}

// Registrar interface can be used to track object's concurrent usage.
//
// Its Register method announces goroutine's intent to use this object's
// facilities; Done method should be called when goroutine finished working with
// this object. Shutdown method stops associated background goroutines so that
// resources can be garbage collected.
//
// These methods provide a similar semantics as sync.WaitGroup's Add(1), and
// Done() methods.
type Registrar interface {
	Register()
	Done()
	Shutdown()
}

// NewSelfCleaningHistogram returns SelfCleaningHistogram wrapping specified
// histogram; its self-cleaning period set to delay.
func NewSelfCleaningHistogram(histogram Histogram, delay time.Duration) *SelfCleaningHistogram {
	ctx, cancel := context.WithCancel(context.Background())
	h := &SelfCleaningHistogram{
		Histogram: histogram,
		c:         make(chan int),
		ctx:       ctx,
		cancel:    cancel,
	}
	// make sure goroutine is started before returning
	guard := make(chan struct{})
	go h.decay(delay, guard)
	<-guard
	return h
}

// decay tracks usage of SelfCleaningHistogram, starting and stopping cleaning
// timer as needed
func (h *SelfCleaningHistogram) decay(delay time.Duration, guard chan<- struct{}) {
	var t *time.Timer
	close(guard)
	var cnt int
	for {
		select {
		case i := <-h.c:
			cnt += i
			switch cnt {
			case 0:
				t = time.AfterFunc(delay, h.Clear)
			default:
				if t != nil {
					t.Stop()
				}
			}
		case <-h.ctx.Done():
			if t != nil {
				t.Stop()
			}
			return
		}
	}
}

// Register implements Registrar interface, using sync.WaitGroup.Add(1) for each
// call, blocking self-cleaning timer until all object's users releases it with
// Done() call.
func (h *SelfCleaningHistogram) Register() {
	select {
	case h.c <- 1:
	case <-h.ctx.Done():
	}
}

// Done implements Registrar interface, using sync.WaitGroup.Done() for each
// call.
func (h *SelfCleaningHistogram) Done() {
	select {
	case h.c <- -1:
	case <-h.ctx.Done():
	}
}

// Shutdown implements Registrar interface, it stops background goroutine. This
// method should be called as the very last method on object and needed only if
// object has to be removed and garbage collected.
func (h *SelfCleaningHistogram) Shutdown() { h.cancel() }
