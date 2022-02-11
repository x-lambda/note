package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
)

func initClientTrace() {
	var reporter jaeger.Reporter
	reporter = jaeger.NewNullReporter()
	// 全量采样
	sampler, _ := jaeger.NewProbabilisticSampler(1)

	// 全局tracer
	// opentracing设置了规范，jaeger是具体的实现方式
	tracer, _ := jaeger.NewTracer(
		"client-test",
		sampler,
		reporter,
		jaeger.TracerOptions.Gen128Bit(true), // 默认是64位trace id
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
	// {'uber-trace-id': '46dd8f7d2260622d16e10c3d13967d48:16e10c3d13967d48:0000000000000000:1'}
	// 格式信息参考: https://www.jaegertracing.io/docs/1.31/client-libraries/#key
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
