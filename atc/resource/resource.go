package resource

import (
	"context"
	"github.com/concourse/concourse/atc/runner"
	"github.com/concourse/concourse/atc/storage"
	"io"
	"path/filepath"

	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . ResourceFactory

type ResourceFactory interface {
	NewResourceForContainer(runnable runner.Runnable) Resource
}

//go:generate counterfeiter . Resource

type Resource interface {
	Get(context.Context, storage.Blob, IOConfig, atc.Source, atc.Params, atc.Version) (VersionedSource, error)
	Put(context.Context, IOConfig, atc.Source, atc.Params) (VersionResult, error)
	Check(context.Context, atc.Source, atc.Version) ([]atc.Version, error)
}

type ResourceType string

type Metadata interface {
	Env() []string
}

type IOConfig struct {
	Stdout io.Writer
	Stderr io.Writer
}

// TODO: check if we need it
func ResourcesDir(suffix string) string {
	return filepath.Join("/tmp", "build", suffix)
}

func NewResource(runnable runner.Runnable) *resource {
	return &resource{
		runnable: runnable,
	}
}

type resource struct {
	//container worker.Container
	runnable runner.Runnable

	ScriptFailure bool
}

func NewResourceFactory() *resourceFactory {
	return &resourceFactory{}
}

type resourceFactory struct{}

func (rf *resourceFactory) NewResourceForContainer(runnable runner.Runnable) Resource {
	return NewResource(runnable)
}
