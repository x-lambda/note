package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/zipkin"
)

func initClientTrace() {
	var reporter jaeger.Reporter
	reporter = jaeger.NewNullReporter()
	// 全量采样
	sampler, _ := jaeger.NewProbabilisticSampler(1)

	// 全局tracer
	// opentracing设置了规范，jaeger是具体的实现方式
	// jaeger默认使用的是Uber-Trace-Id header头，也可以使用 zipkin B3 http header
	propagator := zipkin.NewZipkinB3HTTPHeaderPropagator()
	tracer, _ := jaeger.NewTracer(
		"client-test",
		sampler,
		reporter,
		jaeger.TracerOptions.Injector(opentracing.HTTPHeaders, propagator),
		jaeger.TracerOptions.Extractor(opentracing.HTTPHeaders, propagator),
		jaeger.TracerOptions.ZipkinSharedRPCSpan(true),
	)

	// 设置全局Tracer
	opentracing.SetGlobalTracer(tracer)
}

// RunClient 启动客户端进程
func RunClient() {
	initClientTrace()
	ctx := context.TODO()

	client := &http.Client{}
	req, err := http.NewRequest("GET", "http://127.0.0.1:8080/api/test/uber", nil)
	if err != nil {
		panic(err)
	}

	// 生成一个请求的span
	span, ctx := opentracing.StartSpanFromContext(ctx, "do-http")
	defer span.Finish()

	req = req.WithContext(ctx)
	fmt.Println(GetTraceID(req.Context())) // 输出trace id

	// 把 trace 信息注入到http header中
	// refer to https://opentracing.io/docs/overview/inject-extract/
	opentracing.GlobalTracer().Inject(span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(req.Header))

	// 遍历header发现就是生成了类似下面的信息
	// X-B3-Traceid [454aa5a3c1eeb1d3]
	// X-B3-Spanid [454aa5a3c1eeb1d3]
	// X-B3-Sampled [1]
	for key, value := range req.Header {
		fmt.Println(key, value)
	}

	/**
	 * 更多的封装可以参考：
	 * 		https://github.com/x-lambda/nautilus/blob/master/pkg/xhttp/http.go#L47
	 */

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
