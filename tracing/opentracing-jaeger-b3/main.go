package main

import (
	"context"
	"flag"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
)

// GetTraceID 从span中获取trace id
func GetTraceID(ctx context.Context) string {
	span := opentracing.SpanFromContext(ctx)
	if span == nil {
		return ""
	}

	sc, ok := span.Context().(jaeger.SpanContext)
	if !ok {
		return ""
	}

	return sc.TraceID().String()
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
