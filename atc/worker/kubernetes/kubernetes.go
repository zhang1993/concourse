package kubernetes

import (
	"context"
	"fmt"
	"io"
	"time"

	"code.cloudfoundry.org/garden"
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
	be         *backend.Backend
	wf         db.WorkerFactory
}

func NewClient(
	inCluster bool,
	config, namespace string,
	dbWorkerFactory db.WorkerFactory,
) (k Kubernetes, err error) {
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

	k.wf = dbWorkerFactory

	return
}

var _ worker.Client = Kubernetes{}

func (k Kubernetes) RunCheckStep(
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
) ([]atc.Version, error) {

	//  1. database setup
	//
	//
	// `k8s` comes from the hardcoded registration
	//
	w, found, err := k.wf.GetWorker("k8s")
	if err != nil {
		return nil, fmt.Errorf("get worker: %w")
	}

	if !found {
		return nil, fmt.Errorf("no worker found")
	}

	creating, created, err := w.FindContainer(owner)
	if err != nil {
		return nil, fmt.Errorf("find container: %w")
	}

	var handle string

	switch {
	case creating != nil:
		handle = creating.Handle()
	case created != nil:
		handle = created.Handle()
	default:
		creating, err = w.CreateContainer(
			owner,
			containerMetadata,
		)
		if err != nil {
			return nil, fmt.Errorf("creating db container: %w", err)
		}

		handle = creating.Handle()
	}

	// TODO tell that it's a not found
	container, err := k.be.Lookup(handle)
	if err != nil {
		_, ok := err.(garden.ContainerNotFoundError)
		if !ok {
			return nil, fmt.Errorf("pod lookup: %w", err)
		}
	}

	if created != nil {
		if container == nil {
			// how come?
			return nil, fmt.Errorf("couldn't find pod of container marked as created: %s", handle)
		}
	}

	if container == nil {
		resTypeURI, err := resourceTypeURI(containerSpec.ImageSpec.ResourceType, w)
		if err != nil {
			return nil, fmt.Errorf("resource type to uri: %w", err)
		}

		// create the pod
		container, err = k.be.Create(garden.ContainerSpec{
			Handle: handle,
			Image: garden.ImageRef{
				URI: resTypeURI,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("creating container: %w", err)
		}
	}

	result, err := checkable.Check(
		context.Background(),
		runtime.ProcessSpec{
			Path: "/opt/resource/check",
		},
		container,
	)
	if err != nil {
		return nil, fmt.Errorf("checking: %w", err)
	}

	// actually run the check
	// implement the runner necessary for the Checkable to take care of it

	// perform a lookup, or try to create

	return result, nil
}

func resourceTypeURI(resourceType string, worker db.Worker) (uri string, err error) {
	for _, wrt := range worker.ResourceTypes() {
		if wrt.Type == resourceType {
			uri = wrt.Image
			return
		}
	}

	err = fmt.Errorf("res type '%s' not found", resourceType)
	return
}

func (k Kubernetes) RunTaskStep(
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

func (k Kubernetes) RunPutStep(
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

func (k Kubernetes) RunGetStep(
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

func (k Kubernetes) FindContainer(
	logger lager.Logger, teamID int, handle string,
) (
	container worker.Container, found bool, err error,
) {
	return
}

func (k Kubernetes) FindVolume(
	logger lager.Logger,
	teamID int,
	handle string,
) (vol worker.Volume, found bool, err error) {
	return
}
func (k Kubernetes) CreateVolume(
	logger lager.Logger,
	vSpec worker.VolumeSpec,
	wSpec worker.WorkerSpec,
	volumeType db.VolumeType,
) (vol worker.Volume, err error) {
	return
}
func (k Kubernetes) StreamFileFromArtifact(
	ctx context.Context,
	logger lager.Logger,
	artifact runtime.Artifact,
	filePath string,
) (_ io.ReadCloser, err error) {
	return
}
