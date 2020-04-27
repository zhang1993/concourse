package db

import (
	"encoding/json"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
)

//go:generate counterfeiter . EventProcessor

type EventProcessor interface {
	Initialize(build Build) error
	Process(build Build, event atc.Event) error
	Finalize(build Build) error
}

type buildEventStore struct {
	conn Conn
}

func NewBuildEventStore(conn Conn) *buildEventStore {
	return &buildEventStore{
		conn: conn,
	}
}

func (s *buildEventStore) Initialize(build Build) error {
	_, err := s.conn.Exec(fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventSeq(build.ID())))
	return err
}

func (s *buildEventStore) Finalize(build Build) error {
	_, err := s.conn.Exec(fmt.Sprintf(`
		DROP SEQUENCE %s
	`, buildEventSeq(build.ID())))
	return err
}

func (s *buildEventStore) Process(build Build, event atc.Event) error {
	var err error
	var tx Tx
	tx, err = s.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	err = s.saveEvent(tx, build, event)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return s.conn.Bus().Notify(buildEventsChannel(build.ID()))
}

func (s *buildEventStore) saveEvent(tx Tx, build Build, event atc.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	table := fmt.Sprintf("team_build_events_%d", build.TeamID())
	if build.PipelineID() != 0 {
		table = fmt.Sprintf("pipeline_build_events_%d", build.PipelineID())
	}
	_, err = psql.Insert(table).
		Columns("event_id", "build_id", "type", "version", "payload").
		Values(sq.Expr("nextval('"+buildEventSeq(build.ID())+"')"), build.ID(), string(event.EventType()), string(event.Version()), payload).
		RunWith(tx).
		Exec()
	return err
}

func buildEventSeq(buildid int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildid)
}

func buildEventsChannel(buildID int) string {
	return fmt.Sprintf("build_events_%d", buildID)
}
