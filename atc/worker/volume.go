package worker

import (
	"archive/tar"
	"github.com/DataDog/zstd"
	"io"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Volume

type Volume interface {
	Handle() string
	Path() string

	SetProperty(key string, value string) error
	Properties() (baggageclaim.VolumeProperties, error)

	SetPrivileged(bool) error

	StreamIn(path string, tarStream io.Reader) error
	StreamOut(path string) (io.ReadCloser, error)
	StreamTo(lager.Logger, ArtifactDestination) error
	StreamFile(lager.Logger, string) (io.ReadCloser, error)

	COWStrategy() baggageclaim.COWStrategy

	InitializeResourceCache(db.UsedResourceCache) error
	InitializeTaskCache(logger lager.Logger, jobID int, stepName string, path string, privileged bool) error
	InitializeArtifact(name string, buildID int) (db.WorkerArtifact, error)

	CreateChildForContainer(db.CreatingContainer, string) (db.CreatingVolume, error)

	WorkerName() string
	Destroy() error
}

type VolumeMount struct {
	Volume    Volume
	MountPath string
}

type volume struct {
	bcVolume     baggageclaim.Volume
	dbVolume     db.CreatedVolume
	volumeClient VolumeClient
}

type byMountPath []VolumeMount

func (p byMountPath) Len() int {
	return len(p)
}
func (p byMountPath) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p byMountPath) Less(i, j int) bool {
	path1 := p[i].MountPath
	path2 := p[j].MountPath
	return path1 < path2
}

func NewVolume(
	bcVolume baggageclaim.Volume,
	dbVolume db.CreatedVolume,
	volumeClient VolumeClient,
) Volume {
	return &volume{
		bcVolume:     bcVolume,
		dbVolume:     dbVolume,
		volumeClient: volumeClient,
	}
}

func (v *volume) Handle() string { return v.bcVolume.Handle() }

func (v *volume) Path() string { return v.bcVolume.Path() }

func (v *volume) SetProperty(key string, value string) error {
	return v.bcVolume.SetProperty(key, value)
}

func (v *volume) SetPrivileged(privileged bool) error {
	return v.bcVolume.SetPrivileged(privileged)
}

func (v *volume) StreamIn(path string, tarStream io.Reader) error {
	return v.bcVolume.StreamIn(path, baggageclaim.ZstdEncoding, tarStream)
}

func (v *volume) StreamOut(path string) (io.ReadCloser, error) {
	return v.bcVolume.StreamOut(path, baggageclaim.ZstdEncoding)
}

func (v *volume) Properties() (baggageclaim.VolumeProperties, error) {
	return v.bcVolume.Properties()
}

func (v *volume) WorkerName() string {
	return v.dbVolume.WorkerName()
}

func (v *volume) Destroy() error {
	return v.bcVolume.Destroy()
}

func (v *volume) COWStrategy() baggageclaim.COWStrategy {
	return baggageclaim.COWStrategy{
		Parent: v.bcVolume,
	}
}

func (v *volume) InitializeResourceCache(urc db.UsedResourceCache) error {
	return v.dbVolume.InitializeResourceCache(urc)
}

func (v *volume) InitializeArtifact(name string, buildID int) (db.WorkerArtifact, error) {
	return v.dbVolume.InitializeArtifact(name, buildID)
}

func (v *volume) InitializeTaskCache(
	logger lager.Logger,
	jobID int,
	stepName string,
	path string,
	privileged bool,
) error {
	if v.dbVolume.ParentHandle() == "" {
		return v.dbVolume.InitializeTaskCache(jobID, stepName, path)
	}

	logger.Debug("creating-an-import-volume", lager.Data{"path": v.bcVolume.Path()})

	// always create, if there are any existing task cache volumes they will be gced
	// after initialization of the current one
	importVolume, err := v.volumeClient.CreateVolumeForTaskCache(
		logger,
		VolumeSpec{
			Strategy:   baggageclaim.ImportStrategy{Path: v.bcVolume.Path()},
			Privileged: privileged,
		},
		v.dbVolume.TeamID(),
		jobID,
		stepName,
		path,
	)
	if err != nil {
		return err
	}

	return importVolume.InitializeTaskCache(logger, jobID, stepName, path, privileged)
}

func (v *volume) CreateChildForContainer(creatingContainer db.CreatingContainer, mountPath string) (db.CreatingVolume, error) {
	return v.dbVolume.CreateChildForContainer(creatingContainer, mountPath)
}

func (v *volume) StreamTo(logger lager.Logger, destination ArtifactDestination) error {
	return streamToHelper(v, logger, destination)
}

func (v *volume) StreamFile(logger lager.Logger, path string) (io.ReadCloser, error) {
	return streamFileHelper(v, logger, path)
}

func streamToHelper(s interface {
	StreamOut(string) (io.ReadCloser, error)
}, logger lager.Logger, destination ArtifactDestination) error {
	logger.Debug("start")

	defer logger.Debug("end")

	out, err := s.StreamOut(".")
	if err != nil {
		logger.Error("failed", err)
		return err
	}

	defer out.Close()

	err = destination.StreamIn(".", out)
	if err != nil {
		logger.Error("failed", err)
		return err
	}
	return nil
}

func streamFileHelper(s interface {
	StreamOut(string) (io.ReadCloser, error)
}, logger lager.Logger, path string) (io.ReadCloser, error) {
	out, err := s.StreamOut(path)
	if err != nil {
		return nil, err
	}

	zstdReader := zstd.NewReader(out)
	tarReader := tar.NewReader(zstdReader)

	_, err = tarReader.Next()
	if err != nil {
		return nil, FileNotFoundError{Path: path}
	}

	return fileReadMultiCloser{
		reader: tarReader,
		closers: []io.Closer{
			out,
			zstdReader,
		},
	}, nil
}

type fileReadMultiCloser struct {
	reader  io.Reader
	closers []io.Closer
}

func (frc fileReadMultiCloser) Read(p []byte) (n int, err error) {
	return frc.reader.Read(p)
}

func (frc fileReadMultiCloser) Close() error {
	for _, closer := range frc.closers {
		err := closer.Close()
		if err != nil {
			return err
		}
	}

	return nil
}
