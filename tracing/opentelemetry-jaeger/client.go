package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

// initClientTraceProvider 客户端otel tracing配置
func initClientTraceProvider() {
	endpoint := "127.0.0.1:6831" // udp协议端口
	sampler := 1.0
	batcher := "jaeger"

	exporter, err := createExporter(batcher, endpoint)
	if err != nil {
		panic(err)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampler))),
		sdktrace.WithResource(resource.NewSchemaless(semconv.ServiceNameKey.String("otel-jaeger-client"))),
		sdktrace.WithBatcher(exporter),
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	// TODO 修改这里的Propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		fmt.Printf("[otel] err: %+v\n", err)
	}))
}

// RunClient 启动客户端进程
func RunClient() {
	initClientTraceProvider()
	ctx := context.TODO()

	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://127.0.0.1:8080/api/test/otel", nil)
	if err != nil {
		panic(err)
	}

	tp := otel.GetTracerProvider()
	tracer := tp.Tracer("open-telemetry-go-contrib", trace.WithInstrumentationVersion(SemVersion()))
	ctx, span := tracer.Start(ctx, "do-http")
	defer span.End()

	req = req.WithContext(ctx)
	fmt.Println(GetTraceID(req.Context())) // 输出trace id

	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))

	// 通过打印header可以发现，header中携带了类似的信息
	// Traceparent [00-b5d38b624bcd9425425655706e50ec9b-7113b440efafbb41-01]
	for key, value := range req.Header {
		fmt.Println(key, value)
	}

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	fmt.Println(string(content))
}
