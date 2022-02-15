package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/zipkin"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// otel默认使用w3c规范trace(https://www.w3.org/TR/trace-context/)
// 即在http header头中传递以下字段(如果使用的是http协议跨进程传递):
//     traceparent: 00-0af7651916cd43dd8448eb211c80319c-00f067aa0ba902b7-01
//     tracestate: rojo=00f067aa0ba902b7,congo=t61rcWkgMzE
//
// 其中tracestate是可选字段
// traceparent是必传字段，字段含义
//     |-------------------------------------------------------------------------------------|
//     |    00       -  0af7651916cd43dd8448eb211c80319c - 00f067aa0ba902b7 -      01        |
//     | ${version}  -         ${trace_id}               -   ${parent-id}   - ${trace-flags} |
//     |-------------------------------------------------------------------------------------|
//
// 如果需要使用其他格式/规范header头，例如：
//       jaeger-uber: https://www.jaegertracing.io/docs/1.31/client-libraries/#propagation-format
//       b3: https://github.com/openzipkin/b3-propagation#single-header
//
// 就需要修改otel的context propagation
// otel在扩展包中有提供已经实现的b3 propagation: https://pkg.go.dev/go.opentelemetry.io/contrib/propagators/b3

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

// createExporter 选择存储trace信息的后端，只支持jaeger/zipkin
func createExporter(batcher string, endpoint string) (exporter sdktrace.SpanExporter, err error) {
	switch batcher {
	case "jaeger":
		// 使用udp协议端口
		var opt jaeger.EndpointOption
		config := strings.SplitN(endpoint, ":", 2)
		if len(config) == 2 {
			opt = jaeger.WithAgentEndpoint(jaeger.WithAgentHost(config[0]), jaeger.WithAgentPort(config[1]))
		} else {
			opt = jaeger.WithAgentEndpoint(jaeger.WithAgentHost(config[0]))
		}

		return jaeger.New(opt)
	case "zipkin":
		return zipkin.New(endpoint)
	default:
		return nil, fmt.Errorf("unsupport exporter: %s", batcher)
	}
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
