package worker

import (
	"io"

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
