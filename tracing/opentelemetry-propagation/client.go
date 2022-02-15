package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.opentelemetry.io/contrib/propagators/b3"
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
	// 修改propagation，使用b3头
	// 客户端这里的配置要和服务端一致，不然服务端无法正确的提取trace信息
	otel.SetTextMapPropagator(b3.New(b3.WithInjectEncoding(b3.B3MultipleHeader)))
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
	// X-B3-Traceid [297caf4e448fa39db2223d15f962bbad]
	// X-B3-Spanid [ffc20314dd720c17]
	// X-B3-Sampled [1]
	// 格式参考: https://github.com/openzipkin/b3-propagation#multiple-headers
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
