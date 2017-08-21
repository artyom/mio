package mio

import (
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/artyom/metrics"
)

func BenchmarkRawReader(b *testing.B) {
	file, err := os.Open(os.Args[0])
	if err != nil {
		b.Fatal("failed to open file:", err)
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(fi.Size())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err := io.Copy(ioutil.Discard, file); err != nil {
			b.Fatal("failed to copy data:", err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMioReader(b *testing.B) {
	histogram := metrics.NewHistogram(metrics.NewUniformSample(100))
	file, err := os.Open(os.Args[0])
	if err != nil {
		b.Fatal("failed to open file:", err)
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(fi.Size())
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		mr := NewReader(file, histogram)
		if _, err := io.Copy(ioutil.Discard, mr); err != nil {
			b.Fatal("failed to copy data:", err)
		}
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			b.Fatal(err)
		}
	}
}

func TestReaderBasic(t *testing.T) {
	histogram := metrics.NewHistogram(metrics.NewUniformSample(100))
	file, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal("failed to open file:", err)
	}
	defer file.Close()
	r := io.LimitReader(file, 1<<19)
	mr := NewReader(r, histogram)
	n, err := io.Copy(ioutil.Discard, mr)
	if err != nil {
		t.Fatal("failed to copy data:", err)
	}
	t.Log("bytes copied:", n)
	t.Logf("%d reads, latency min: %s, max: %s",
		histogram.Count(),
		time.Duration(histogram.Min()),
		time.Duration(histogram.Max()))
	if histogram.Count() == 0 {
		t.Fatal("histogram should have some registered samples")
	}
}

func TestReaderSelfCleaning(t *testing.T) {
	histogram := NewSelfCleaningHistogram(
		metrics.NewHistogram(metrics.NewUniformSample(100)),
		150*time.Millisecond)
	file, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal("failed to open file:", err)
	}
	defer file.Close()
	r := io.LimitReader(file, 1<<19)
	mr := NewReader(r, histogram)
	n, err := io.Copy(ioutil.Discard, mr)
	if err != nil {
		t.Fatal("failed to copy data:", err)
	}
	if err := mr.Close(); err != nil {
		t.Fatal("metered reader close error:", err)
	}
	t.Log("bytes copied:", n)
	t.Logf("%d reads, latency min: %s, max: %s",
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

func TestReaderDoubleClose(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatal("double close caused panic:", r)
		}
	}()
	histogram := NewSelfCleaningHistogram(
		metrics.NewHistogram(metrics.NewUniformSample(100)),
		150*time.Millisecond)
	file, err := os.Open(os.Args[0])
	if err != nil {
		t.Fatal("failed to open file:", err)
	}
	mr := NewReader(file, histogram)
	t.Log("testing double Close(), should call Done() on underlying Registrar only once")
	mr.Close()
	mr.Close()
}
