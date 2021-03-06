package mio

import (
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/artyom/metrics"
)

func TestWriterDoubleClose(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatal("double close caused panic:", r)
		}
	}()
	histogram := NewSelfCleaningHistogram(
		metrics.NewHistogram(metrics.NewUniformSample(100)),
		150*time.Millisecond)
	mw := NewWriter(ioutil.Discard, histogram)
	t.Log("testing double Close(), should call Done() on underlying Registrar only once")
	mw.Close()
	mw.Close()
}

func TestWriterBasic(t *testing.T) {
	histogram := metrics.NewHistogram(metrics.NewUniformSample(100))
	mw := NewWriter(ioutil.Discard, histogram)
	file, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal("failed to open file:", err)
	}
	defer file.Close()
	r := io.LimitReader(file, 1<<19)
	n, err := io.Copy(mw, r)
	if err != nil {
		t.Fatal("failed to copy data:", err)
	}
	t.Log("bytes copied:", n)
	t.Logf("%d writes, latency min: %s, max: %s",
		histogram.Count(),
		time.Duration(histogram.Min()),
		time.Duration(histogram.Max()))
	if histogram.Count() == 0 {
		t.Fatal("histogram should have some registered samples")
	}
}

func TestWriterSelfCleaning(t *testing.T) {
	histogram := NewSelfCleaningHistogram(
		metrics.NewHistogram(metrics.NewUniformSample(100)),
		150*time.Millisecond)
	mw := NewWriter(ioutil.Discard, histogram)
	file, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal("failed to open file:", err)
	}
	defer file.Close()
	r := io.LimitReader(file, 1<<19)
	n, err := io.Copy(mw, r)
	if err != nil {
		t.Fatal("failed to copy data:", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatal("metered writer close error:", err)
	}
	t.Log("bytes copied:", n)
	t.Logf("%d writes, latency min: %s, max: %s",
		histogram.Count(),
		time.Duration(histogram.Min()),
		time.Duration(histogram.Max()))
	if histogram.Count() == 0 {
		t.Fatal("histogram should have some registered samples")
	}
	t.Log("waiting for released histogram to clear")
	time.Sleep(200 * time.Millisecond)
	cnt := histogram.Count()
	t.Logf("%d writes, latency min: %s, max: %s",
		cnt,
		time.Duration(histogram.Min()),
		time.Duration(histogram.Max()))
	if cnt != 0 {
		t.Fatal("histogram should be empty, but has samples:", cnt)
	}
}

func TestSelfCleaningHistogram(t *testing.T) {
	sh := NewSelfCleaningHistogram(
		metrics.NewHistogram(metrics.NewUniformSample(100)),
		150*time.Millisecond)
	t.Log("registering activity")
	sh.Register()
	sh.Register()
	sh.Update(150)
	sh.Update(100)
	sh.Update(50)
	sh.Done()
	sh.Done()
	t.Log("activity de-registered")
	if cnt := sh.Count(); cnt != 3 {
		t.Fatal("should have 3 registered samples, got:", cnt)
	}
	t.Log("waiting for histogram to clear")
	time.Sleep(300 * time.Millisecond)
	if cnt := sh.Count(); cnt != 0 {
		t.Fatal("should have 0 registered samples, got:", cnt)
	}
	t.Log("registering activity")
	sh.Register()
	sh.Update(50)
	t.Log("waiting for period longer than clear delay")
	time.Sleep(300 * time.Millisecond)
	sh.Update(150)
	if cnt := sh.Count(); cnt != 2 {
		t.Fatal("should have 2 registered samples, got:", cnt)
	}
}

func TestSelfCleaningHistogram_Shutdown(t *testing.T) {
	sh := NewSelfCleaningHistogram(
		metrics.NewHistogram(metrics.NewUniformSample(100)),
		100*time.Millisecond)
	t.Log("registering activity")
	sh.Register()
	sh.Register()
	sh.Update(150)
	sh.Update(100)
	sh.Update(50)
	sh.Done()
	sh.Done()
	t.Log("activity de-registered")
	if cnt := sh.Count(); cnt != 3 {
		t.Fatal("should have 3 registered samples, got:", cnt)
	}
	sh.Shutdown()
	sh.Shutdown()
	t.Log("waiting for period longer than clear delay, timer should be stopped")
	time.Sleep(200 * time.Millisecond)
	if cnt := sh.Count(); cnt != 3 {
		t.Fatal("should have 3 registered samples, got:", cnt)
	}
}

func TestSelfCleaningHistogram_concurrent(t *testing.T) {
	if runtime.NumCPU() < 2 {
		t.Skip("NumCPU<2, skipping: this test requires parallelism")
	}
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	sh := NewSelfCleaningHistogram(
		metrics.NewHistogram(metrics.NewUniformSample(100)),
		100*time.Millisecond)
	defer sh.Shutdown()
	const c = 10000
	var wg sync.WaitGroup
	wg.Add(c)
	for i := 0; i < c; i++ {
		go func() {
			sh.Register()
			sh.Update(100)
			sh.Done()
			wg.Done()
		}()
	}
	wg.Wait()
}
