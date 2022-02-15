package main

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	tracerKey = "otel-go-contrib-tracer"
)

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

// initServerTraceProvider 服务端otel tracing配置
// 开发环境可以一键部署jaeger:
// docker run -d -p6831:6831/udp -p16686:16686 -p14268:14268 jaegertracing/all-in-one:latest
func initServerTraceProvider() {
	endpoint := "127.0.0.1:6831" // udp协议端口
	sampler := 1.0
	batcher := "jaeger"

	exporter, err := createExporter(batcher, endpoint)
	if err != nil {
		panic(err)
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampler))),
		sdktrace.WithResource(resource.NewSchemaless(semconv.ServiceNameKey.String("otel-jaeger-server"))),
		sdktrace.WithBatcher(exporter),
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	// TODO 修改propagation
	// 默认的跨进程，通过http header头传递trace信息
	// trace格式 Traceparent [00-b5d38b624bcd9425425655706e50ec9b-7113b440efafbb41-01]
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		fmt.Printf("[otel] err: %+v\n", err)
	}))
}

// NewTraceID gin框架中间件
func NewTraceID() gin.HandlerFunc {
	tp := otel.GetTracerProvider()
	tracer := tp.Tracer("open-telemetry-go-contrib", trace.WithInstrumentationVersion(SemVersion()))

	return func(c *gin.Context) {
		c.Set(tracerKey, tracer)
		savedCtx := c.Request.Context()
		defer func() {
			c.Request = c.Request.WithContext(savedCtx)
		}()

		ctx := otel.GetTextMapPropagator().Extract(savedCtx, propagation.HeaderCarrier(c.Request.Header))
		opts := []trace.SpanStartOption{
			trace.WithAttributes(semconv.NetAttributesFromHTTPRequest("tcp", c.Request)...),
			trace.WithAttributes(semconv.EndUserAttributesFromHTTPRequest(c.Request)...),
			trace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest("ServeHTTP", c.FullPath(), c.Request)...),
			trace.WithSpanKind(trace.SpanKindServer),
		}

		spanName := c.FullPath()
		if spanName == "" {
			spanName = fmt.Sprintf("HTTP %s route not found", c.Request.Method)
		}

		ctx, span := tracer.Start(ctx, spanName, opts...)
		defer span.End()

		traceID := GetTraceID(ctx)
		// 打印trace id
		// fmt.Println(traceID)
		// TODO 把trace信息注入到ctx中
		c.Writer.Header().Set("x-trace-id", traceID)

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		status := c.Writer.Status()
		attrs := semconv.HTTPAttributesFromHTTPStatusCode(status)
		spanStatus, spanMessage := semconv.SpanStatusFromHTTPStatusCode(status)
		span.SetAttributes(attrs...)
		span.SetStatus(spanStatus, spanMessage)

		if len(c.Errors) > 0 {
			errStr := c.Errors.String()
			span.RecordError(fmt.Errorf(errStr))
			span.SetStatus(codes.Error, errStr)
		}
	}
}

// RunServer 启动服务端进程
func RunServer() {
	// 初始化trace服务配置
	initServerTraceProvider()

	r := gin.Default()
	// 中间件，每个请求进来的时候都会经过中间件处理
	r.Use(NewTraceID())

	r.GET("/api/test/otel", func(c *gin.Context) {
		// 通过请求上下文对象Context, 直接往客户端返回一个json
		c.JSON(200, gin.H{"message": "ok"})
	})

	r.Run()
}
