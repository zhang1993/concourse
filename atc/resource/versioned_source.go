package resource

import (
	"io"
	"path"

	"code.cloudfoundry.org/garden"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/worker"
)

//go:generate counterfeiter . VersionedSource

type VersionedSource interface {
	Version() atc.Version
	Metadata() []atc.MetadataField

	StreamOut(string) (io.ReadCloser, error)
	StreamIn(string, io.Reader) error

	Artifact() worker.Artifact
}

type versionResult struct {
	Version atc.Version `json:"version"`

	Metadata []atc.MetadataField `json:"metadata,omitempty"`
}

type putVersionedSource struct {
	versionResult versionResult

	container garden.Container

	resourceDir string
}

func (vs *putVersionedSource) Version() atc.Version {
	return vs.versionResult.Version
}

func (vs *putVersionedSource) Metadata() []atc.MetadataField {
	return vs.versionResult.Metadata
}

func (vs *putVersionedSource) StreamOut(src string) (io.ReadCloser, error) {
	return vs.container.StreamOut(garden.StreamOutSpec{
		// don't use path.Join; it strips trailing slashes
		Path: vs.resourceDir + "/" + src,
	})
}

func (vs *putVersionedSource) Artifact() worker.Artifact {
	return nil
}

func (vs *putVersionedSource) StreamIn(dst string, src io.Reader) error {
	return vs.container.StreamIn(garden.StreamInSpec{
		Path:      path.Join(vs.resourceDir, dst),
		TarStream: src,
	})
}

func NewGetVersionedSource(artifact worker.Artifact, version atc.Version, metadata []atc.MetadataField) VersionedSource {
	return &getVersionedSource{
		artifact:    artifact,
		resourceDir: ResourcesDir("get"),

		versionResult: versionResult{
			Version:  version,
			Metadata: metadata,
		},
	}
}

type getVersionedSource struct {
	versionResult versionResult

	artifact    worker.Artifact
	resourceDir string
}

func (vs *getVersionedSource) Version() atc.Version {
	return vs.versionResult.Version
}

func (vs *getVersionedSource) Metadata() []atc.MetadataField {
	return vs.versionResult.Metadata
}

func (vs *getVersionedSource) StreamOut(src string) (io.ReadCloser, error) {
	readCloser, err := vs.artifact.StreamOut(src)
	if err != nil {
		return nil, err
	}

	return readCloser, err
}

func (vs *getVersionedSource) StreamIn(dst string, src io.Reader) error {
	return vs.artifact.StreamIn(
		path.Join(vs.resourceDir, dst),
		src,
	)
}

func (vs *getVersionedSource) Artifact() worker.Artifact {
	return vs.artifact
}
