package pipelineserver

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/concourse/concourse/atc/db"
)

type Server struct {
	logger                lager.Logger
	teamFactory           db.TeamFactory
	rejector              auth.Rejector
	pipelineFactory       db.PipelineFactory
	buildCreator          db.BuildCreator
	externalURL           string
	enableArchivePipeline bool
}

func NewServer(
	logger lager.Logger,
	teamFactory db.TeamFactory,
	pipelineFactory db.PipelineFactory,
	buildCreator db.BuildCreator,
	externalURL string,
	enableArchivePipeline bool,
) *Server {
	return &Server{
		logger:                logger,
		teamFactory:           teamFactory,
		rejector:              auth.UnauthorizedRejector{},
		pipelineFactory:       pipelineFactory,
		buildCreator:          buildCreator,
		externalURL:           externalURL,
		enableArchivePipeline: enableArchivePipeline,
	}
}
