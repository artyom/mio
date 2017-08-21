package mio

import (
	"io"
	"time"
)

// Reader wraps io.Reader and registers each read operation latency in attached
// histogram
type Reader struct {
	io.Reader
	h      Histogram
	closed bool
}

// NewReader attaches provided histogram to reader, returning new
// io.Reader. If histogram implements Registrar interface, this would also call
// its Register() method.
func NewReader(reader io.Reader, h Histogram) *Reader {
	mr := &Reader{
		Reader: reader,
		h:      h,
	}
	if r, ok := h.(Registrar); ok {
		r.Register()
	}
	return mr
}

// Read implements io.Reader interface; each read operation is timed and sampled
// in attached histogram. Samples are stored in nanoseconds.
func (mr *Reader) Read(p []byte) (n int, err error) {
	var start time.Time
	if mr.h != nil {
		start = time.Now()
	}
	n, err = mr.Reader.Read(p)
	if n > 0 && mr.h != nil {
		mr.h.Update(time.Since(start).Nanoseconds())
	}
	return n, err
}

// Close implements io.Closer interface. If underlying reader implements
// io.Closer, calling this method would also close it. If attached histogram
// also implements Registrar interface, this would call its Done() method.
func (mr *Reader) Close() error {
	if mr.closed {
		return nil
	}
	mr.closed = true
	if r, ok := mr.h.(Registrar); ok {
		r.Done()
	}
	if c, ok := mr.Reader.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
