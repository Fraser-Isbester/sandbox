package main

import (
	"context"
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "example",
		Short: "A CLI with a loading spinner and OTel tracing",
		Run: func(cmd *cobra.Command, args []string) {
			tracer, shutdown := setupTracing()
			defer shutdown()

			ctx := context.Background()
			ctx, span := tracer.Start(ctx, "main-task")
			defer span.End()

			s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
			s.Start()
			defer s.Stop()

			// Simulate a task
			if err := doTask(ctx, tracer); err != nil {
				span.RecordError(err)
				fmt.Println("Error:", err)
				return
			}

			fmt.Println("Task complete!")
		},
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

func doTask(ctx context.Context, tracer trace.Tracer) error {
	ctx, span := tracer.Start(ctx, "do-task")
	defer span.End()

	time.Sleep(3 * time.Second) // Simulate work

	return nil // Replace with an error to test error recording
}

func setupTracing() (trace.Tracer, func()) {
	// Create stdout exporter (for now, dumping to stderr)
	exporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
	if err != nil {
		panic(fmt.Sprintf("Failed to create exporter: %v", err))
	}

	// Create a trace provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(provider)

	// Return tracer and shutdown function
	return otel.Tracer("example-cli"), func() {
		_ = provider.Shutdown(context.Background())
	}
}
