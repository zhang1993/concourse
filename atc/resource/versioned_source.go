package resource

import (
	"errors"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"io"
)

//go:generate counterfeiter . VersionedSource

type VersionedSource interface {
	Version() atc.Version
	Metadata() []atc.MetadataField

	StreamOut(string) (io.ReadCloser, error)
	StreamIn(string, io.Reader) error

	Volume() worker.Volume
	VolumeHandle() string
}

func NewGetVersionedSource(volume worker.Volume, version atc.Version, metadata []atc.MetadataField) VersionedSource {
	return &getVersionedSource{
		volumeHandle: volume.Handle(),
		resourceDir:  ResourcesDir("get"),

		versionResult: runtime.VersionResult{
			Version:  version,
			Metadata: metadata,
		},
	}
}

type getVersionedSource struct {
	versionResult runtime.VersionResult

	volumeHandle string
	resourceDir  string
}

func (vs *getVersionedSource) Version() atc.Version {
	return vs.versionResult.Version
}

func (vs *getVersionedSource) Metadata() []atc.MetadataField {
	return vs.versionResult.Metadata
}

func (vs *getVersionedSource) StreamOut(src string) (io.ReadCloser, error) {
	//readCloser, err := vs.volume.StreamOut(src)
	//if err != nil {
	//	return nil, err
	//}
	//
	//return readCloser, err
	return nil, errors.New("getVersionedSource.StreamOut not implemented")
}

func (vs *getVersionedSource) StreamIn(dst string, src io.Reader) error {
	//return vs.volume.StreamIn(
	//	path.Join(vs.resourceDir, dst),
	//	src,
	//)
	return errors.New("getVersionedSource.StreamIn not implemented")
}

func (vs *getVersionedSource) Volume() worker.Volume {
	//return vs.volume
	return nil
}

func (vs *getVersionedSource) VolumeHandle() string {
	return vs.volumeHandle
}
