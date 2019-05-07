package worker

import (
	"errors"

	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/db"
)

var ErrMissingVolume = errors.New("artifact mounted to container is missing")

//go:generate counterfeiter . Container

type Container interface {
	garden.Container

	Destroy() error

	ArtifactMounts() []ArtifactMount

	WorkerName() string

	MarkAsHijacked() error
}

type gardenWorkerContainer struct {
	garden.Container
	dbContainer db.CreatedContainer
	dbVolumes   []db.CreatedVolume

	gardenClient garden.Client

	artifactMounts []ArtifactMount

	user       string
	workerName string
}

func newGardenWorkerContainer(
	logger lager.Logger,
	container garden.Container,
	dbContainer db.CreatedContainer,
	dbContainerVolumes []db.CreatedVolume,
	gardenClient garden.Client,
	volumeClient VolumeClient,
	workerName string,
) (Container, error) {
	logger = logger.WithData(lager.Data{"container": container.Handle()})

	workerContainer := &gardenWorkerContainer{
		Container:   container,
		dbContainer: dbContainer,
		dbVolumes:   dbContainerVolumes,

		gardenClient: gardenClient,

		workerName: workerName,
	}

	err := workerContainer.initializeVolumes(logger, volumeClient)
	if err != nil {
		return nil, err
	}

	properties, err := workerContainer.Properties()
	if err != nil {
		return nil, err
	}

	if properties["user"] != "" {
		workerContainer.user = properties["user"]
	} else {
		workerContainer.user = "root"
	}

	return workerContainer, nil
}

func (container *gardenWorkerContainer) Destroy() error {
	return container.gardenClient.Destroy(container.Handle())
}

func (container *gardenWorkerContainer) WorkerName() string {
	return container.workerName
}

func (container *gardenWorkerContainer) MarkAsHijacked() error {
	return container.dbContainer.MarkAsHijacked()
}

func (container *gardenWorkerContainer) Run(spec garden.ProcessSpec, io garden.ProcessIO) (garden.Process, error) {
	spec.User = container.user
	return container.Container.Run(spec, io)
}

func (container *gardenWorkerContainer) ArtifactMounts() []ArtifactMount {
	return container.artifactMounts
}

func (container *gardenWorkerContainer) initializeVolumes(
	logger lager.Logger,
	volumeClient VolumeClient,
) error {

	artifactMounts := []ArtifactMount{}

	for _, dbVolume := range container.dbVolumes {
		volumeLogger := logger.Session("artifact", lager.Data{
			"handle": dbVolume.Handle(),
		})

		volume, volumeFound, err := volumeClient.LookupVolume(logger, dbVolume.Handle())
		if err != nil {
			volumeLogger.Error("failed-to-lookup-artifact", err)
			return err
		}

		if !volumeFound {
			volumeLogger.Error("artifact-is-missing-on-worker", ErrMissingVolume, lager.Data{"handle": dbVolume.Handle()})
			return errors.New("artifact mounted to container is missing " + dbVolume.Handle() + " from worker " + container.workerName)
		}

		artifactMounts = append(artifactMounts, ArtifactMount{
			Artifact:  volume,
			MountPath: dbVolume.Path(),
		})
	}

	container.artifactMounts = artifactMounts

	return nil
}
