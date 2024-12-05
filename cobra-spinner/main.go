package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"github.com/tmc/spinner"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// SpinnerWriter wraps an io.Writer and manages spinner state during writes
type SpinnerWriter struct {
	out     io.Writer
	spinner *spinner.Spinner
	mu      sync.Mutex
}

func NewSpinnerWriter(out io.Writer, s *spinner.Spinner) *SpinnerWriter {
	return &SpinnerWriter{
		out:     out,
		spinner: s,
	}
}

// Write implements io.Writer and manages spinner state
func (w *SpinnerWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.spinner != nil {
		w.spinner.Stop()
		defer w.spinner.Start()
	}
	return w.out.Write(p)
}

// CLI represents our command-line application
type CLI struct {
	spinner *spinner.Spinner
	stdout  *SpinnerWriter
	stderr  *SpinnerWriter
}

func NewCLI() *CLI {
	s := spinner.New()

	return &CLI{
		spinner: s,
		stdout:  NewSpinnerWriter(os.Stdout, s),
		stderr:  NewSpinnerWriter(os.Stderr, s),
	}
}

func (c *CLI) Start() {
	c.spinner.Start()
}

func (c *CLI) Stop() {
	c.spinner.Stop()
}

func (c *CLI) Println(a ...interface{}) {
	fmt.Fprintln(c.stdout, a...)
}

func (c *CLI) Printf(format string, a ...interface{}) {
	fmt.Fprintf(c.stdout, format, a...)
}

func (c *CLI) Errorln(a ...interface{}) {
	fmt.Fprintln(c.stderr, a...)
}

func main() {
	cli := NewCLI()

	var rootCmd = &cobra.Command{
		Use:   "example",
		Short: "A CLI with a loading spinner and OTel tracing",
		Run: func(cmd *cobra.Command, args []string) {
			tracer, shutdown := setupTracing()
			defer shutdown()

			cli.Start()
			defer cli.Stop()

			ctx := context.Background()
			ctx, span := tracer.Start(ctx, "main-task")
			defer span.End()

			cli.Println("Starting task...")

			if err := doTask(ctx, tracer, cli); err != nil {
				span.RecordError(err)
				cli.Errorln("Error:", err)
				return
			}

			cli.Println("Task complete!")
		},
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

// doTask simulates a long-running operation that we want to trace
func doTask(ctx context.Context, tracer trace.Tracer, cli *CLI) error {
	_, span := tracer.Start(ctx, "do-task")
	defer span.End()

	cli.Errorln("Doing some work...")
	time.Sleep(3 * time.Second) // Simulate work

	return nil
}

// setupTracing configures OpenTelemetry tracing
func setupTracing() (trace.Tracer, func()) {
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		panic(fmt.Sprintf("Failed to create exporter: %v", err))
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(provider)

	return otel.Tracer("example-cli"), func() {
		_ = provider.Shutdown(context.Background())
	}
}
