package worker

import (
	"io"

	"code.cloudfoundry.org/lager"

	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Artifact

type Artifact interface {
	Store(string, io.Reader) error
	Retrieve(string) (io.ReadCloser, error)
	Volume() Volume
	Initialized() bool
}

type artifact struct {
	dbArtifact db.WorkerArtifact
	volume     Volume
}

func NewArtifact(
	dbArtifact db.WorkerArtifact,
	volume Volume,
) Artifact {
	return &artifact{
		dbArtifact: dbArtifact,
		volume:     volume,
	}
}

func (a *artifact) Volume() Volume {
	return a.volume
}

// I did this so our tests don't need to make a tree of fakes
func (a *artifact) Initialized() bool {
	return a.dbArtifact.Initialized()
}

func (a *artifact) Store(path string, reader io.Reader) error {
	return a.volume.StreamIn(path, reader)
}

func (a *artifact) Retrieve(path string) (io.ReadCloser, error) {
	return a.volume.StreamOut(path)
}

type artifactManager struct {
	artifactProvider db.ArtifactProvider
	volumeClient     VolumeClient
}

func NewArtifactManager(lifecycle db.ArtifactProvider, client VolumeClient) ArtifactManager {
	return &artifactManager{
		artifactProvider: lifecycle,
		volumeClient:     client,
	}
}

func (manager *artifactManager) FindArtifactForResourceCache(logger lager.Logger, resourceCache db.UsedResourceCache) (Artifact, bool, error) {
	dbArtifact, err := manager.artifactProvider.FindArtifactForResourceCache(logger, resourceCache.ID())

	if err != nil {
		return nil, false, err
	}

	if dbArtifact == nil {
		return nil, false, nil
	}

	volume, err := manager.volumeClient.FindVolumeForArtifact(logger, dbArtifact.ID())

	if err != nil {
		return nil, false, err
	}

	if volume == nil {
		return nil, false, nil
	}

	return NewArtifact(dbArtifact, volume), true, nil
}

func (manager *artifactManager) FindArtifactForTaskCache(lager.Logger, int, int, string, string) (Artifact, bool, error) {
	return nil, false, nil
}

func (manager *artifactManager) CertsArtifact(lager.Logger) (artifact Artifact, found bool, err error) {
	return nil, false, nil
}

func (manager *artifactManager) LookupArtifact(lager.Logger, string) (Artifact, bool, error) {
	return nil, false, nil
}

func (manager *artifactManager) CreateVolumeForArtifact(logger lager.Logger, teamID int, artifactID int) (Volume, error) {
	return nil, nil
}
