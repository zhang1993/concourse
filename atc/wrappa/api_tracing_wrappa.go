package wrappa

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/tracing"
	"github.com/tedsuo/rata"
)

type APITracingWrappa struct{}

func NewAPITracingWrappa(logger lager.Logger) Wrappa {
	return APITracingWrappa{}
}

func (wrappa APITracingWrappa) Wrap(handlers rata.Handlers) rata.Handlers {
	var wrapped = rata.Handlers{}

	for name, handler := range handlers {
		switch name {
		case atc.BuildEvents, atc.DownloadCLI, atc.HijackContainer:
			wrapped[name] = handler
		default:
			wrapped[name] = tracing.WrapHandler(name, handler)
		}
	}

	return wrapped
}
