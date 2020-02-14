package kubernetes

import (
	"context"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/resource"
	"github.com/concourse/concourse/atc/runtime"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/kubernetes/backend"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Kubernetes struct {
	Namespace  string `long:"namespace"`
	InCluster  bool   `long:"in-cluster"`
	Kubeconfig string `long:"config" default:"~/.kube/config"`

	be *backend.Backend
}

func NewClient(inCluster bool, config, namespace string) (k Kubernetes, err error) {
	var cfg *rest.Config

	switch {
	case config != "":
		cfg, err = clientcmd.BuildConfigFromFlags("", config)
		if err != nil {
			return
		}
	case inCluster:
		cfg, err = rest.InClusterConfig()
		if err != nil {
			err = fmt.Errorf("incluster cfg: %w", err)
			return
		}
	default:
		err = fmt.Errorf("incluster or config must be specified")
		return
	}

	k.be, err = backend.New(namespace, cfg)
	if err != nil {
		err = fmt.Errorf("new backend: %w")
		return
	}

	return
}

var _ worker.Client = Kubernetes{}

// func (c Kubernetes) findOrCreateContainer(ctx context.Context) (container SoMeThInG, err error) {
// 	return
// }

func (c Kubernetes) RunCheckStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec worker.ContainerSpec,
	workerSpec worker.WorkerSpec,
	strategy worker.ContainerPlacementStrategy,
	containerMetadata db.ContainerMetadata,
	resourceTypes atc.VersionedResourceTypes,
	timeout time.Duration,
	checkable resource.Resource,
) (result []atc.Version, err error) {
	containerMetadata.StepName

	// 1. find or create the container
	// 2. create a checkable, and check

	return
}

func (c Kubernetes) RunTaskStep(
	ctx context.Context,
	logger lager.Logger,
	owner db.ContainerOwner,
	containerSpec worker.ContainerSpec,
	workerSpec worker.WorkerSpec,
	strategy worker.ContainerPlacementStrategy,
	metadata db.ContainerMetadata,
	imageFetcherSpec worker.ImageFetcherSpec,
	processSpec runtime.ProcessSpec,
	eventDelegate runtime.StartingEventDelegate,
	lockFactory lock.LockFactory,
) (result worker.TaskResult, err error) {
	// TODO inputs
	// TODO caches
	// TODO non-(docker-image|resgistry-image) image resource
	// TODO image from artifact

	err = fmt.Errorf("not implemented")

	// err = c.createContainer(ctx, containerSpec)
	// if err != nil {
	// 	return
	// }

	return
}

func (c Kubernetes) RunPutStep(
	context.Context,
	lager.Logger,
	db.ContainerOwner,
	worker.ContainerSpec,
	worker.WorkerSpec,
	worker.ContainerPlacementStrategy,
	db.ContainerMetadata,
	worker.ImageFetcherSpec,
	runtime.ProcessSpec,
	runtime.StartingEventDelegate,
	resource.Resource,
) (result worker.PutResult, err error) {
	err = fmt.Errorf("not implemented")
	return
}

func (c Kubernetes) RunGetStep(
	context.Context,
	lager.Logger,
	db.ContainerOwner,
	worker.ContainerSpec,
	worker.WorkerSpec,
	worker.ContainerPlacementStrategy,
	db.ContainerMetadata,
	worker.ImageFetcherSpec,
	runtime.ProcessSpec,
	runtime.StartingEventDelegate,
	db.UsedResourceCache,
	resource.Resource,
) (result worker.GetResult, err error) {
	err = fmt.Errorf("not implemented")
	return
}

func (c Kubernetes) FindContainer(
	logger lager.Logger, teamID int, handle string,
) (
	container worker.Container, found bool, err error,
) {
	return
}

func (c Kubernetes) FindVolume(
	logger lager.Logger,
	teamID int,
	handle string,
) (vol worker.Volume, found bool, err error) {
	return
}
func (c Kubernetes) CreateVolume(
	logger lager.Logger,
	vSpec worker.VolumeSpec,
	wSpec worker.WorkerSpec,
	volumeType db.VolumeType,
) (vol worker.Volume, err error) {
	return
}
func (c Kubernetes) StreamFileFromArtifact(
	ctx context.Context,
	logger lager.Logger,
	artifact runtime.Artifact,
	filePath string,
) (_ io.ReadCloser, err error) {
	return
}
