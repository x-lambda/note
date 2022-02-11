package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/zipkin"
)

const StartedSpanOPName = "StartHTTPRequest"

// initServerTrace 设置服务端的trace组件
// 通用的方案有jaeger/zipkin/SkyWalking等
// 如果没有调用opentracing.SetGlobalTracer(tracer)
// 则不会生成trace信息(trace_id,span_id,sample)
func initServerTrace() {
	var reporter jaeger.Reporter
	// 这里使用了NewNullReporter，并没有真正的上报到jaeger存储
	reporter = jaeger.NewNullReporter()
	// 全量采样
	sampler, _ := jaeger.NewProbabilisticSampler(1)

	// 全局tracer
	// opentracing设置了规范，jaeger是具体的实现方式
	// jaeger默认使用的是Uber-Trace-Id header头，也可以使用 zipkin B3 http header
	// 注意这里创建的方式要和客户端一致，否则客户端传递过来的trace信息，服务端解析不了
	//
	// 这里使用B3头的原因是在service mesh中有的支持
	// Envoy 使用 Zipkin 风格头信息
	// https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/observability/tracing
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

// OpentracingTraceID  中间件，用于后台服务的链路追踪
func OpentracingTraceID() gin.HandlerFunc {
	return func(c *gin.Context) {
		var span opentracing.Span
		c.Request, span = startSpan(c.Request)
		defer span.Finish()

		c.Next()
	}
}

func startSpan(req *http.Request) (spanReq *http.Request, span opentracing.Span) {
	ctx := req.Context()

	tracer := opentracing.GlobalTracer()
	carrier := opentracing.HTTPHeadersCarrier(req.Header)

	// 如果能在header中提取出trace信息，则创建一个子span，相当于续用原有的trace信息
	// 如果没提取出trace信息，则创建一个新的root span
	if spanCtx, err := tracer.Extract(opentracing.HTTPHeaders, carrier); err == nil {
		span = opentracing.StartSpan(StartedSpanOPName, opentracing.ChildOf(spanCtx))
		ctx = opentracing.ContextWithSpan(ctx, span)
	} else {
		span, ctx = opentracing.StartSpanFromContext(ctx, StartedSpanOPName)
	}

	// 将trace id信息注入到ctx中
	traceID := GetTraceID(ctx)
	fmt.Println("opentracing trace id: ", traceID)

	// 这里只是把trace id存放到context中
	// 后续的handler和业务处理函数，第一个参数都应该是ctx
	// 可以参考https://github.com/x-lambda/nautilus框架处理方式
	ctx = context.WithValue(ctx, "trace_id", traceID)
	spanReq = req.WithContext(ctx)
	return
}

// RunServer 启动服务端进程
func RunServer() {
	initServerTrace()

	r := gin.Default()
	// 中间件，每个请求进来的时候都会经过中间件处理
	r.Use(OpentracingTraceID())

	r.GET("/api/test/uber", func(c *gin.Context) {
		// 通过请求上下文对象Context, 直接往客户端返回一个json
		c.JSON(200, gin.H{"message": "ok"})
	})

	r.Run()
}
