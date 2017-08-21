package mio

import (
	"io"
	"time"
)

// Writer wraps io.Writer and registers each write operation latency in
// attached histogram
type Writer struct {
	io.Writer
	h      Histogram
	closed bool
}

// NewWriter attaches provided histogram to writer, returning new
// io.Writer. If histogram implements Registrar interface, this would also call
// its Register() method.
func NewWriter(writer io.Writer, h Histogram) *Writer {
	mw := &Writer{
		Writer: writer,
		h:      h,
	}
	if r, ok := h.(Registrar); ok {
		r.Register()
	}
	return mw
}

// Write implements io.Writer interface; each write operation is timed and
// sampled in attached histogram. Samples are stored in nanoseconds.
func (mw *Writer) Write(p []byte) (n int, err error) {
	var start time.Time
	if mw.h != nil {
		start = time.Now()
	}
	n, err = mw.Writer.Write(p)
	if n > 0 && mw.h != nil {
		mw.h.Update(time.Since(start).Nanoseconds())
	}
	return n, err
}

// Close implements io.Closer interface. If underlying writer implements
// io.Closer, calling this method would also close it. If attached histogram
// also implements Registrar interface, this would call its Done() method.
func (mw *Writer) Close() error {
	if mw.closed {
		return nil
	}
	mw.closed = true
	if r, ok := mw.h.(Registrar); ok {
		r.Done()
	}
	if c, ok := mw.Writer.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
