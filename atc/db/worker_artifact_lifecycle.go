package db

import (
	"database/sql"
	"time"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . ArtifactCreator

type ArtifactCreator interface {
	CreateArtifact(name string) (WorkerArtifact, error)
}

//go:generate counterfeiter . WorkerArtifactLifecycle

type WorkerArtifactLifecycle interface {
	ArtifactCreator
	RemoveExpiredArtifacts(lager.Logger) error
	RemoveOrphanedArtifacts(lager.Logger) error
}

type artifactLifecycle struct {
	conn Conn
}

func NewArtifactLifecycle(conn Conn) *artifactLifecycle {
	return &artifactLifecycle{
		conn: conn,
	}
}

func (lifecycle *artifactLifecycle) CreateArtifact(name string) (WorkerArtifact, error) {
	var artifactID int
	var createdAt time.Time
	err := psql.Insert("worker_artifacts").
		Columns("name").
		Values(name).
		Suffix("RETURNING id, created_at").
		RunWith(lifecycle.conn).
		QueryRow().
		Scan(&artifactID, &createdAt)

	if err != nil {
		return nil, err
	}

	return &artifact{
		conn:      lifecycle.conn,
		id:        artifactID,
		createdAt: createdAt,
		name:      name,
	}, nil
}

func (lifecycle *artifactLifecycle) RemoveExpiredArtifacts(logger lager.Logger) error {

	_, err := psql.Delete("worker_artifacts").
		Where(sq.Expr("created_at < NOW() - interval '12 hours'")).
		Where(sq.Eq{"initialized": false}).
		RunWith(lifecycle.conn).
		Exec()

	if err != nil {
		return err
	}

	return nil
}

func (lifecycle *artifactLifecycle) RemoveOrphanedArtifacts(logger lager.Logger) error {
	err := lifecycle.markArtifactsOwnedByTerminatedBuilds()
	if err != nil {
		logger.Error("could not mark terminated build artifacts", err)
		return err
	}

	query, args, err := psql.Delete("worker_artifacts").
		Where(
			sq.Eq{
				"initialized":                  true,
				"build_id":                     sql.NullInt64{},
				"worker_resource_cache_id":     nil,
				"worker_task_cache_id":         nil,
				"worker_base_resource_type_id": nil,
				"worker_resource_certs_id":     nil,
			}).
		RunWith(lifecycle.conn).
		ToSql()

	if err != nil {
		return err
	}

	rows, err := lifecycle.conn.Query(query, args...)
	if err != nil {
		return err
	}

	defer Close(rows)
	return nil
}

// TODO: Change this to keep around artifacts required for hijackable builds
func (lifecycle *artifactLifecycle) markArtifactsOwnedByTerminatedBuilds() error {
	query, args, err := psql.Select("a.id", "a.initialized", "b.status").
		From("worker_artifacts a").
		LeftJoin("builds b ON b.id = a.build_id").
		Where(
			sq.Eq{"initialized": true},
			sq.NotEq{"a.build_id": sql.NullInt64{}},
		).
		RunWith(lifecycle.conn).
		ToSql()

	if err != nil {
		return err
	}

	rows, err := lifecycle.conn.Query(query, args...)
	if err != nil {
		return err
	}

	defer Close(rows)

	var artifactsWithTerminatedBuilds []int

	for rows.Next() {
		var (
			id          int
			initialized bool
			buildStatus sql.NullString
		)
		err = rows.Scan(&id, &initialized, &buildStatus)

		if err != nil {
			return err
		}

		if initialized && isTerminated(buildStatus) {
			artifactsWithTerminatedBuilds = append(artifactsWithTerminatedBuilds, id)
		}
	}

	idOrClause := sq.Or{}

	for _, id := range artifactsWithTerminatedBuilds {
		idOrClause = append(idOrClause, sq.Eq{"id": id})
	}

	_, err = psql.Update("worker_artifacts").Set("build_id", sql.NullInt64{}).Where(idOrClause).RunWith(lifecycle.conn).Exec()

	return err
}

func isTerminated(buildStatus sql.NullString) bool {
	return buildStatus.Valid && buildStatus.String != string(BuildStatusPending) && buildStatus.String != string(BuildStatusStarted)
}
