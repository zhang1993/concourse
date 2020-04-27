package buildserver

import (
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db"
)

type EventHandlerFactory func(lager.Logger, db.Build) http.Handler

type Server struct {
	logger lager.Logger

	externalURL string

	teamFactory         db.TeamFactory
	buildFactory        db.BuildFactory
	buildCreator        db.BuildCreator
	eventHandlerFactory EventHandlerFactory
	rejector            auth.Rejector
}

func NewServer(
	logger lager.Logger,
	externalURL string,
	teamFactory db.TeamFactory,
	buildFactory db.BuildFactory,
	buildCreator db.BuildCreator,
	eventHandlerFactory EventHandlerFactory,
) *Server {
	return &Server{
		logger: logger,

		externalURL: externalURL,

		teamFactory:         teamFactory,
		buildFactory:        buildFactory,
		buildCreator:        buildCreator,
		eventHandlerFactory: eventHandlerFactory,

		rejector: auth.UnauthorizedRejector{},
	}
}
