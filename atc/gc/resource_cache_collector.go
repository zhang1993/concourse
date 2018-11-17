package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/opentracing/opentracing-go"
)

type resourceCacheCollector struct {
	cacheLifecycle db.ResourceCacheLifecycle
}

func NewResourceCacheCollector(cacheLifecycle db.ResourceCacheLifecycle) Collector {
	return &resourceCacheCollector{
		cacheLifecycle: cacheLifecycle,
	}
}

func (rcc *resourceCacheCollector) Run(ctx context.Context) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "resource-cache-collector")
	defer span.Finish()

	logger := lagerctx.FromContext(ctx).Session("resource-cache-collector")
	logger.Debug("start")
	defer logger.Debug("done")

	return rcc.cacheLifecycle.CleanUpInvalidCaches(logger)
}
