package plugin

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/events/plugin/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
)

func buildMsg(build db.Build) *proto.Build {
	return &proto.Build{
		Id:          int32(build.ID()),
		Name:        build.Name(),
		JobId:       int32(build.JobID()),
		JobName:     build.JobName(),
		TeamId:      int32(build.TeamID()),
		TeamName:    build.TeamName(),
		Status:      string(build.Status()),
		StartTime:   timestampMsg(build.StartTime()),
		EndTime:     timestampMsg(build.EndTime()),
		IsCompleted: build.IsCompleted(),
		RerunOf:     int32(build.RerunOf()),
		RerunOfName: build.RerunOfName(),
		RerunNumber: int32(build.RerunNumber()),
	}
}

func pipelineMsg(pipeline db.Pipeline) *proto.Pipeline {
	return &proto.Pipeline{
		Id:       int32(pipeline.ID()),
		Name:     pipeline.Name(),
		TeamId:   int32(pipeline.TeamID()),
		TeamName: pipeline.TeamName(),
	}
}

func teamMsg(team db.Team) *proto.Team {
	return &proto.Team{
		Id:   int32(team.ID()),
		Name: team.Name(),
	}
}

func timestampMsg(t time.Time) *timestamp.Timestamp {
	if t.IsZero() {
		return nil
	}
	return &timestamp.Timestamp{
		Seconds: t.Unix(),
		Nanos:   int32(t.Nanosecond()),
	}
}

func eventMsg(evt atc.Event) (*proto.Event, error) {
	payload, err := json.Marshal(evt)
	if err != nil {
		return nil, fmt.Errorf("marshal event: %w", err)
	}
	return &proto.Event{
		Event:   string(evt.EventType()),
		Version: string(evt.Version()),
		Data:    payload,
	}, nil
}

func keyMsg(k db.EventKey) *proto.Key {
	key, ok := k.(Key)
	if !ok {
		return nil
	}
	return &proto.Key{Values: key}
}
