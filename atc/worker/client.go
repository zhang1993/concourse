package worker

import (
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"

	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Client

type Client interface {
	FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error)
	FindVolume(logger lager.Logger, teamID int, handle string) (Artifact, bool, error)
	CreateArtifact(logger lager.Logger, name string) (Artifact, error)
	Store(logger lager.Logger, teamID int, artifact atc.WorkerArtifact, volumePath string, data io.ReadCloser) error
}

func NewClient(pool Pool, workerProvider WorkerProvider, artifactProvider db.ArtifactProvider) *client {
	return &client{
		pool:             pool,
		workerProvider:   workerProvider,
		artifactProvider: artifactProvider,
	}
}

type client struct {
	pool             Pool
	workerProvider   WorkerProvider
	artifactProvider db.ArtifactProvider
}

func (client *client) FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error) {
	worker, found, err := client.workerProvider.FindWorkerForContainer(
		logger.Session("find-worker"),
		teamID,
		handle,
	)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return worker.FindContainerByHandle(logger, teamID, handle)
}

func (client *client) FindVolume(logger lager.Logger, teamID int, handle string) (Artifact, bool, error) {
	worker, found, err := client.workerProvider.FindWorkerForVolume(
		logger.Session("find-worker"),
		teamID,
		handle,
	)
	if err != nil {
		return nil, false, err
	}

	if !found {
		return nil, false, nil
	}

	return worker.LookupVolume(logger, handle)
}

func (client *client) CreateArtifact(logger lager.Logger, name string) (Artifact, error) {
	art, err := client.artifactProvider.CreateArtifact(name)
	if err != nil {
		logger.Error("failed-to-create-artifact", err, lager.Data{"name": name})
		return nil, err
	}
	return &artifact{dbArtifact: art, pool: client.pool}, nil
}

func (client *client) Store(logger lager.Logger, teamID int, artifact atc.WorkerArtifact, volumePath string, data io.ReadCloser) error {
}
