package plugin

import (
	"context"

	"github.com/concourse/concourse/atc/events/plugin/proto"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

type GRPCPlugin struct {
	plugin.Plugin
	ServerImpl proto.EventStoreServer
}

func (p *GRPCPlugin) GRPCClient(ctx context.Context, broker *plugin.GRPCBroker, c *grpc.ClientConn) (interface{}, error) {
	return proto.NewEventStoreClient(c), nil
}

func (p *GRPCPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	proto.RegisterEventStoreServer(s, p.ServerImpl)
	return nil
}
