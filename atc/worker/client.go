package worker

import (
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Client

type Client interface {
	FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error)
	FindVolume(logger lager.Logger, teamID int, handle string) (Volume, bool, error)
	CreateArtifact(logger lager.Logger, teamID int, name string) (db.WorkerArtifact, Volume, error)
}

func NewClient(pool Pool, provider WorkerProvider) *client {
	return &client{
		pool:     pool,
		provider: provider,
	}
}

type client struct {
	pool     Pool
	provider WorkerProvider
}

func (client *client) FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error) {
	worker, found, err := client.provider.FindWorkerForContainer(
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

func (client *client) FindVolume(logger lager.Logger, teamID int, handle string) (Volume, bool, error) {
	worker, found, err := client.provider.FindWorkerForVolume(
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

func (client *client) CreateArtifact(logger lager.Logger, teamID int, name string) (db.WorkerArtifact, Volume, error) {
	worker, err := client.pool.FindOrChooseWorker(logger, WorkerSpec{TeamID: teamID})
	if err != nil {
		return nil, nil, err
	}

	return worker.CreateArtifact(logger, teamID, name)

}
