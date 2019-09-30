package runner

import (
	"code.cloudfoundry.org/garden"
	"code.cloudfoundry.org/lager"
	"context"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/storage"
)

type Client interface {
	FindBlobForResourceCache(logger lager.Logger, resourceCache db.UsedResourceCache) (storage.Blob, bool, error)
}

type Properties map[string]string

type TTYSpec struct {
	WindowSize *WindowSize `json:"window_size,omitempty"`
}

type WindowSize struct {
	Columns int `json:"columns,omitempty"`
	Rows    int `json:"rows,omitempty"`
}

type Signal int

type Process interface {
	ID() string
	Wait() (int, error)
	SetTTY(TTYSpec) error
	Signal(Signal) error
}

type Runnable interface {
	Properties() (garden.Properties, error)
	Attach(ctx context.Context, processID string, io garden.ProcessIO) (Process, error)
	Run(context.Context, garden.ProcessSpec, garden.ProcessIO) (Process, error)
	SetProperty(name string, value string) error
	Stop(kill bool) error

	//Equivalent to Container implementing Volume :(
	storage.Blob

}