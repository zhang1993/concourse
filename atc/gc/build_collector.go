package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/opentracing/opentracing-go"
)

type buildCollector struct {
	buildFactory buildFactory
}

type buildFactory interface {
	MarkNonInterceptibleBuilds(ctx context.Context) error
}

func NewBuildCollector(buildFactory buildFactory) *buildCollector {
	return &buildCollector{
		buildFactory: buildFactory,
	}
}

func (b *buildCollector) Run(ctx context.Context) error {
	logger := lagerctx.FromContext(ctx).Session("build-collector")

	span, ctx := opentracing.StartSpanFromContext(ctx, "build-collector")
	defer span.Finish()

	logger.Debug("start")
	defer logger.Debug("done")

	return b.buildFactory.MarkNonInterceptibleBuilds(ctx)
}
