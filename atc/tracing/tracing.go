package tracing

import (
	"fmt"
	"io"
	"net/http"

	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/transport/zipkin"
)

type tracer struct {
	tracer opentracing.Tracer
	closer io.Closer
}

func (t *tracer) Close() error {
	return t.Close()
}

type TracingHandler struct{}

func WrapHandler(name string, handler http.Handler) http.Handler {
	var (
		currentTracer = opentracing.GlobalTracer()
		namingOpt     = nethttp.OperationNameFunc(func(r *http.Request) string {
			return r.Method + " " + name
		})
	)

	return nethttp.MiddlewareFunc(
		currentTracer,
		handler.ServeHTTP,
		namingOpt)
}

func NewTracer(zipkinUrl, component string) (*tracer, error) {
	tracerTransport, err := zipkin.NewHTTPTransport(
		zipkinUrl,
		zipkin.HTTPBatchSize(1),
		zipkin.HTTPLogger(jaeger.StdLogger),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"couldn't initialize http transport: %v", err)
	}

	t, closer := jaeger.NewTracer(
		component,
		jaeger.NewConstSampler(true), // collect every trace
		jaeger.NewRemoteReporter(tracerTransport),
	)

	opentracing.SetGlobalTracer(t)

	return &tracer{
		tracer: t,
		closer: closer,
	}, nil
}
