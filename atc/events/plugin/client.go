package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagerctx"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	"github.com/concourse/concourse/atc/events/plugin/proto"
	"github.com/hashicorp/go-plugin"
)

const pluginEnvPrefix = "CONCOURSE_EVENTSTORE_PLUGIN_"

const Plugin = "eventstore"

type Key []int64

func (k Key) Marshal() ([]byte, error) {
	return json.Marshal(k)
}

func (k Key) GreaterThan(o db.EventKey) bool {
	other, ok := o.(Key)
	if !ok {
		return false
	}
	if len(k) != len(other) {
		return false
	}
	for i := 0; i < len(k); i++ {
		if k[i] > other[i] {
			return true
		}
	}
	return false
}

var Handshake = plugin.HandshakeConfig{
	// The ProtocolVersion is the version that must match between Concourse
	// core and EventStore plugins. This should be bumped whenever a change
	// happens in one or the other that makes it so that they can't safely
	// communicate. This could be adding a new interface value, it could be
	// how helper/schema computes diffs, etc.
	ProtocolVersion: 1,

	// The magic cookie values should NEVER be changed.
	MagicCookieKey:   "CONCOURSE_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "7Ljp84POQB+S9a/8FQo1kD+Ygn7+wvr6UCtRwJKhHndgt0RlAKO9az96mbkRQ6Jv",
}

type Client struct {
	client      *plugin.Client
	storeClient proto.EventStoreClient

	Path string `long:"path" description:"Path of the binary for a gRPC EventStore plugin."`

	logger lager.Logger
}

func (c *Client) IsConfigured() bool {
	return c.Path != ""
}

func (c *Client) Setup(ctx context.Context) error {
	c.logger = lagerctx.FromContext(ctx)

	c.client = plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig:  Handshake,
		Plugins:          plugin.PluginSet{Plugin: &GRPCPlugin{}},
		Cmd:              c.pluginCmd(),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		AutoMTLS:         true,
	})

	rpcClient, err := c.client.Client()
	if err != nil {
		c.logger.Error("connect-to-client", err)
		return fmt.Errorf("connect to client: %w", err)
	}

	raw, err := rpcClient.Dispense(Plugin)
	if err != nil {
		c.logger.Error("dispense-plugin", err)
		return fmt.Errorf("dispense plugin: %w", err)
	}

	c.storeClient = raw.(proto.EventStoreClient)
	_, err = c.storeClient.Setup(ctx, &proto.Setup_Request{})
	if err != nil {
		c.logger.Error("setup", err)
		return fmt.Errorf("setup: %w", err)
	}

	return nil
}

func (c *Client) pluginCmd() *exec.Cmd {
	cmd := exec.Command(c.Path)
	cmd.Env = detectPluginEnv(c.logger)

	return cmd
}

func (c *Client) Close(ctx context.Context) error {
	defer c.client.Kill()

	_, err := c.storeClient.Close(ctx, &proto.Close_Request{})
	if err != nil {
		c.logger.Error("close", err)
		return fmt.Errorf("close: %w", err)
	}

	return nil
}

func (c *Client) Initialize(ctx context.Context, build db.Build) error {
	_, err := c.storeClient.Initialize(ctx, &proto.Initialize_Request{
		Build: buildMsg(build),
	})
	if err != nil {
		c.logger.Error("initialize", err)
		return fmt.Errorf("initialize: %w", err)
	}
	return nil
}

func (c *Client) Finalize(ctx context.Context, build db.Build) error {
	_, err := c.storeClient.Finalize(ctx, &proto.Finalize_Request{
		Build: buildMsg(build),
	})
	if err != nil {
		c.logger.Error("finalize", err)
		return fmt.Errorf("finalize: %w", err)
	}
	return nil
}

func (c *Client) Put(ctx context.Context, build db.Build, events []atc.Event) (db.EventKey, error) {
	protoEvents := make([]*proto.Event, len(events))
	for i, evt := range events {
		var err error
		protoEvents[i], err = eventMsg(evt)
		if err != nil {
			c.logger.Error("marshal-event", err)
			return nil, err
		}
	}
	r, err := c.storeClient.Put(ctx, &proto.Put_Request{
		Build:  buildMsg(build),
		Events: protoEvents,
	})
	if err != nil {
		c.logger.Error("put", err)
		return nil, fmt.Errorf("put: %w", err)
	}
	return Key(r.Key.Values), nil
}

func (c *Client) Get(ctx context.Context, build db.Build, requested int, cursor *db.EventKey) ([]event.Envelope, error) {
	r, err := c.storeClient.Get(ctx, &proto.Get_Request{
		Build:     buildMsg(build),
		Requested: int32(requested),
		Cursor:    keyMsg(*cursor),
	})
	if err != nil {
		c.logger.Error("get", err)
		return nil, fmt.Errorf("get: %w", err)
	}
	*cursor = Key(r.Cursor.Values)
	evts := make([]event.Envelope, len(r.Events))
	for i, protoEvent := range r.Events {
		evts[i] = toEnvelope(protoEvent)
	}
	return evts, nil
}

func (c *Client) Delete(ctx context.Context, builds []db.Build) error {
	protoBuilds := make([]*proto.Build, len(builds))
	for i, build := range builds {
		protoBuilds[i] = buildMsg(build)
	}
	_, err := c.storeClient.Delete(ctx, &proto.Delete_Request{
		Builds: protoBuilds,
	})
	if err != nil {
		c.logger.Error("delete", err)
		return fmt.Errorf("delete: %w", err)
	}
	return nil
}

func (c *Client) DeletePipeline(ctx context.Context, pipeline db.Pipeline) error {
	_, err := c.storeClient.DeletePipeline(ctx, &proto.DeletePipeline_Request{
		Pipeline: pipelineMsg(pipeline),
	})
	if err != nil {
		c.logger.Error("delete-pipeline", err)
		return fmt.Errorf("delete pipeline: %w", err)
	}
	return nil
}

func (c *Client) DeleteTeam(ctx context.Context, team db.Team) error {
	_, err := c.storeClient.DeleteTeam(ctx, &proto.DeleteTeam_Request{
		Team: teamMsg(team),
	})
	if err != nil {
		c.logger.Error("delete-team", err)
		return fmt.Errorf("delete team: %w", err)
	}
	return nil
}

func (c *Client) UnmarshalKey(data []byte, key *db.EventKey) error {
	var target Key
	if err := json.Unmarshal(data, &target); err != nil {
		return err
	}
	*key = target
	return nil
}

func toEnvelope(protoEvent *proto.Event) event.Envelope {
	raw := json.RawMessage(protoEvent.Data)
	return event.Envelope{
		Data:    &raw,
		Event:   atc.EventType(protoEvent.Event),
		Version: atc.EventVersion(protoEvent.Version),
	}
}
