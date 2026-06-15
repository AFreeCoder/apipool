package reqlog

import (
	"bytes"
	"sync"

	"github.com/gin-gonic/gin"
)

type CaptureWriter struct {
	gin.ResponseWriter
	limit     int
	buf       bytes.Buffer
	truncated bool
}

var captureWriterPool = sync.Pool{
	New: func() any {
		return &CaptureWriter{}
	},
}

func AcquireCaptureWriter(rw gin.ResponseWriter, limit int) *CaptureWriter {
	w, ok := captureWriterPool.Get().(*CaptureWriter)
	if !ok || w == nil {
		w = &CaptureWriter{}
	}
	w.ResponseWriter = rw
	w.limit = limit
	w.truncated = false
	w.buf.Reset()
	return w
}

func ReleaseCaptureWriter(w *CaptureWriter) {
	if w == nil {
		return
	}
	w.ResponseWriter = nil
	w.limit = 0
	w.truncated = false
	w.buf.Reset()
	captureWriterPool.Put(w)
}

func (w *CaptureWriter) Write(b []byte) (int, error) {
	w.capture(b)
	return w.ResponseWriter.Write(b)
}

func (w *CaptureWriter) WriteString(s string) (int, error) {
	w.captureString(s)
	return w.ResponseWriter.WriteString(s)
}

func (w *CaptureWriter) Flush() {
	w.ResponseWriter.Flush()
}

func (w *CaptureWriter) CapturedCopy() []byte {
	if w == nil {
		return nil
	}
	return cloneBytes(w.buf.Bytes())
}

func (w *CaptureWriter) Truncated() bool {
	return w != nil && w.truncated
}

func (w *CaptureWriter) capture(b []byte) {
	if w == nil || w.limit <= 0 || len(b) == 0 {
		if w != nil && len(b) > 0 && w.limit <= 0 {
			w.truncated = true
		}
		return
	}
	remaining := w.limit - w.buf.Len()
	if remaining <= 0 {
		w.truncated = true
		return
	}
	if len(b) > remaining {
		_, _ = w.buf.Write(b[:remaining])
		w.truncated = true
		return
	}
	_, _ = w.buf.Write(b)
}

func (w *CaptureWriter) captureString(s string) {
	if w == nil || w.limit <= 0 || len(s) == 0 {
		if w != nil && len(s) > 0 && w.limit <= 0 {
			w.truncated = true
		}
		return
	}
	remaining := w.limit - w.buf.Len()
	if remaining <= 0 {
		w.truncated = true
		return
	}
	if len(s) > remaining {
		_, _ = w.buf.WriteString(s[:remaining])
		w.truncated = true
		return
	}
	_, _ = w.buf.WriteString(s)
}
