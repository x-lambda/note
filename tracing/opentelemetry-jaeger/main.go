package main

import (
	"context"
	"flag"

	"go.opentelemetry.io/otel/trace"
)

// GetTraceID 提取trace id
func GetTraceID(ctx context.Context) (traceID string) {
	traceID = "no-trace-id"

	if span := trace.SpanContextFromContext(ctx); span.HasTraceID() {
		traceID = span.TraceID().String()
	}

	return
}

func version() string {
	return "0.0.1"
}

// SemVersion is the semantic version to be supplied to tracer/meter creation.
func SemVersion() string {
	return "semver:" + version()
}

// 先启动服务端 go run main.go [--process=server]
// 再启动客户端 go run main.go --process=client
func main() {
	process := "server"
	flag.StringVar(&process, "process", "server", "client or server")
	flag.Parse()

	if process == "server" {
		RunServer()
	} else {
		RunClient()
	}
}
