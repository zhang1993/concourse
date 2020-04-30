package db

import (
	"encoding/json"
	"fmt"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"strconv"
	"strings"
)

//go:generate counterfeiter . EventProcessor

type EventProcessor interface {
	Initialize(build Build) error
	Process(build Build, event atc.Event) error
	Finalize(build Build) error
}

//go:generate counterfeiter . EventStore

type EventStore interface {
	EventProcessor

	Delete(buildIDs ...int) error
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

func (s *buildEventStore) Delete(buildIDs ...int) error {
	if len(buildIDs) == 0 {
		return nil
	}

	interfaceBuildIDs := make([]interface{}, len(buildIDs))
	for i, buildID := range buildIDs {
		interfaceBuildIDs[i] = buildID
	}

	indexStrings := make([]string, len(buildIDs))
	for i := range indexStrings {
		indexStrings[i] = "$" + strconv.Itoa(i+1)
	}

	tx, err := s.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	_, err = tx.Exec(`
   DELETE FROM build_events
	 WHERE build_id IN (`+strings.Join(indexStrings, ",")+`)
	 `, interfaceBuildIDs...)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		UPDATE builds
		SET reap_time = now()
		WHERE id IN (`+strings.Join(indexStrings, ",")+`)
	`, interfaceBuildIDs...)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func buildEventSeq(buildid int) string {
	return fmt.Sprintf("build_event_id_seq_%d", buildid)
}

func buildEventsChannel(buildID int) string {
	return fmt.Sprintf("build_events_%d", buildID)
}
