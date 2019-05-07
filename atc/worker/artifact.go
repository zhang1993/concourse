package worker

import (
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/concourse/atc/db"
)

//go:generate counterfeiter . Artifact

type Artifact interface {
	ID() int
	Name() string
	BuildID() int
	CreatedAt() time.Time
	//TODO: Remove this!
	DBArtifact() db.WorkerArtifact
	// TODO: Do we need this after volumes become artifacts?
	Handle() string // used in worker.createVolumes for inputs/outputs
	Path() string   // used for cert volume mounts

	SetProperty(key string, value string) error
	Properties() (baggageclaim.VolumeProperties, error)

	SetPrivileged(bool) error

	StreamIn(path string, tarStream io.Reader) error
	StreamOut(path string) (io.ReadCloser, error)

	COWStrategy() baggageclaim.COWStrategy

	InitializeResourceCache(db.UsedResourceCache) error
	InitializeTaskCache(lager.Logger, int, string, string, bool) error
	InitializeArtifact(name string, buildID int) (db.WorkerArtifact, error)

	CreateChildForContainer(db.CreatingContainer, string) (db.CreatingVolume, error)

	WorkerName() string
	Destroy() error
	AttachVolume(bcVolume baggageclaim.Volume, dbVolume db.CreatedVolume, client VolumeClient)
	Store(logger lager.Logger, teamID int, volumePath string, data io.ReadCloser) error

	// create a volume on another worker or cow if same worker, streamIn the data and return the newly created artifact
	// Clone() baggageclaim.Volume
	//	 if self on Same worker:
	//		 newVol := self.VolumeClient.FindOrCreateCOW
	//	 else:
	//		 newVol := self.VolumeClient.FindOrCreate(EmptyStrategy)
	//		 newVol.StreamIn(self.StreamOut("/"))
	// 	return newVol
}

type VolumeMount struct {
	Volume    baggageclaim.Volume
	MountPath string
}

type ArtifactMount struct {
	Artifact  Artifact
	MountPath string
}

type mounts []ArtifactMount

func (m mounts) Order() {

}

type artifact struct {
	dbArtifact   db.WorkerArtifact
	bcVolume     baggageclaim.Volume
	dbVolume     db.CreatedVolume
	pool         Pool
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

func NewArtifact(
	dbArtifact db.WorkerArtifact,
	bcVolume baggageclaim.Volume,
	dbVolume db.CreatedVolume,
	client VolumeClient,
) Artifact {
	return &artifact{
		dbArtifact:   dbArtifact,
		bcVolume:     bcVolume,
		dbVolume:     dbVolume,
		volumeClient: client,
	}
}

func (a *artifact) AttachVolume(bcVolume baggageclaim.Volume, dbVolume db.CreatedVolume, client VolumeClient) {
	a.dbVolume = dbVolume
	a.bcVolume = bcVolume
	a.volumeClient = client
}

func (a *artifact) Store(logger lager.Logger, teamID int, volumePath string, data io.ReadCloser) error {
	worker, err := a.pool.FindOrChooseWorkerForArtifact(logger, WorkerSpec{TeamID: teamID}, a.ID())
	if err != nil {
		logger.Error("failed-to-find-worker-for-artifact", err, lager.Data{"artifactID": a.ID()})
		return err
	}

	spec := VolumeSpec{
		Strategy: baggageclaim.EmptyStrategy{},
	}
	err = worker.FindOrCreateVolume(logger, spec, teamID, a, db.VolumeTypeArtifact)
	if err != nil {
		return err
	}

	return a.StreamIn(volumePath, data)
}

func (a *artifact) DBArtifact() db.WorkerArtifact {
	return a.dbArtifact
}

func (a *artifact) ID() int {
	return a.dbArtifact.ID()
}

func (a *artifact) Name() string {
	return a.dbArtifact.Name()
}

func (a *artifact) BuildID() int {
	return a.dbArtifact.BuildID()
}

func (a *artifact) CreatedAt() time.Time {
	return a.dbArtifact.CreatedAt()
}

func (a *artifact) Handle() string { return a.bcVolume.Handle() }

func (a *artifact) Path() string { return a.bcVolume.Path() }

func (a *artifact) SetProperty(key string, value string) error {
	return a.bcVolume.SetProperty(key, value)
}

func (a *artifact) SetPrivileged(privileged bool) error {
	return a.bcVolume.SetPrivileged(privileged)
}

func (a *artifact) StreamIn(path string, tarStream io.Reader) error {
	return a.bcVolume.StreamIn(path, tarStream)
}

func (a *artifact) StreamOut(path string) (io.ReadCloser, error) {
	return a.bcVolume.StreamOut(path)
}

func (a *artifact) Properties() (baggageclaim.VolumeProperties, error) {
	return a.bcVolume.Properties()
}

func (a *artifact) WorkerName() string {
	return a.dbVolume.WorkerName()
}

func (a *artifact) Destroy() error {
	return a.bcVolume.Destroy()
}

func (a *artifact) COWStrategy() baggageclaim.COWStrategy {
	return baggageclaim.COWStrategy{
		Parent: a.bcVolume,
	}
}

func (a *artifact) InitializeResourceCache(urc db.UsedResourceCache) error {
	return a.dbVolume.InitializeResourceCache(urc)
}

func (a *artifact) InitializeArtifact(name string, buildID int) (db.WorkerArtifact, error) {
	return a.dbVolume.InitializeArtifact(name, buildID)
}

func (a *artifact) InitializeTaskCache(
	logger lager.Logger,
	jobID int,
	stepName string,
	path string,
	privileged bool,
) error {
	if a.dbVolume.ParentHandle() == "" {
		return a.dbVolume.InitializeTaskCache(jobID, stepName, path)
	}

	logger.Debug("creating-an-import-artifact", lager.Data{"path": a.bcVolume.Path()})

	// always create, if there are any existing task cache volumes they will be gced
	// after initialization of the current one
	importVolume, err := a.volumeClient.CreateVolumeForTaskCache(
		logger,
		VolumeSpec{
			Strategy:   baggageclaim.ImportStrategy{Path: a.bcVolume.Path()},
			Privileged: privileged,
		},
		a.dbVolume.TeamID(),
		jobID,
		stepName,
		path,
	)
	if err != nil {
		return err
	}

	return importVolume.InitializeTaskCache(logger, jobID, stepName, path, privileged)
}

func (a *artifact) CreateChildForContainer(creatingContainer db.CreatingContainer, mountPath string) (db.CreatingVolume, error) {
	return a.dbVolume.CreateChildForContainer(creatingContainer, mountPath)
}
