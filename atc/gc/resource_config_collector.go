package gc

import (
	"context"

	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc/db"
	"github.com/opentracing/opentracing-go"
)

type resourceConfigCollector struct {
	configFactory db.ResourceConfigFactory
}

func NewResourceConfigCollector(configFactory db.ResourceConfigFactory) Collector {
	return &resourceConfigCollector{
		configFactory: configFactory,
	}
}

func (rcuc *resourceConfigCollector) Run(ctx context.Context) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "resource-config-collector")
	defer span.Finish()

	logger := lagerctx.FromContext(ctx).Session("resource-config-collector")
	logger.Debug("start")
	defer logger.Debug("done")

	return rcuc.configFactory.CleanUnreferencedConfigs()
}
