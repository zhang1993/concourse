package worker

import (
	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Client

type Client interface {
	FindContainer(logger lager.Logger, teamID int, handle string) (Container, bool, error)
	FindVolume(logger lager.Logger, teamID int, handle string) (Volume, bool, error)
	CreateArtifact(logger lager.Logger, teamID int, name string) (Artifact, error)
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
	artifactProvider db.ArtifactProvider
	volumeClient volumeClient
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

func (client *client) CreateArtifact(logger lager.Logger, teamID int, name string) (Artifact, error) {
	worker, err := client.pool.FindOrChooseWorker(logger, WorkerSpec{TeamID: teamID})
	if err != nil {
		return nil, err
	}

	artifact, err := worker.CreateArtifact(logger, name)
	if err != nil {
		return nil, err
	}

	volume, err := worker.CreateVolumeForArtifact(logger, teamID, artifact.ID())
	if err != nil {
		return nil, err
	}
	return NewArtifact(artifact, volume), nil
}

func doRunStep() {
	// create any artifacts from the resource type ( for a get step )
	// pick a worker
	// create volumes for artifacts on worker
	// create container
	// run get

	// task
	// lookup any artifacts from the build artifact repo thingy
	// pick a worker
	// create container
	// run config

	// put
	// lookup any artifact
	// pick a worker
	// create container
	// run put

	// find or create worker artifacts( db artifact)
	// pick a worker
	// find or create volumes for artifacts on worker
	// create container
	// run the hting

}

// caller:
//	creating artifacts
