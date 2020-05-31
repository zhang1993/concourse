package plugin

import (
	"github.com/concourse/concourse/atc/events"
	"github.com/jessevdk/go-flags"
)

func init() {
	events.Register("plugin", ClientFactory{})
}

type ClientFactory struct {
}

func (s ClientFactory) AddConfig(group *flags.Group) events.Store {
	client := &Client{}

	subGroup, err := group.AddGroup("Plugin Event Store", "", client)
	if err != nil {
		panic(err)
	}

	subGroup.Namespace = "eventstore-plugin"

	return client
}
