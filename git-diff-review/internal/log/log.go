package log

import (
	"io"
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/tmc/spinner"
)

type Writer struct {
	mu      sync.Mutex
	writers []io.Writer
	spinner *spinner.Spinner
	active  bool
}

func NewWriter(writers ...io.Writer) *Writer {
	s := spinner.New()
	return &Writer{
		writers: writers,
		spinner: s,
	}
}

func (w *Writer) StartSpinner() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.active {
		w.spinner.Start()
		w.active = true
	}
}

func (w *Writer) StopSpinner() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.active {
		w.spinner.Stop()
		w.active = false
	}
}

func (w *Writer) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, writer := range w.writers {
		n, err = writer.Write(p)
		if err != nil {
			return n, err
		}
	}
	return len(p), nil
}

func NewLogger(writer *Writer) zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	return zerolog.New(writer).With().Timestamp().Logger()
}

var Logger zerolog.Logger
var WriterInstance *Writer

func init() {
	writer := NewWriter(os.Stderr)
	WriterInstance = writer
	Logger = NewLogger(writer)
}
