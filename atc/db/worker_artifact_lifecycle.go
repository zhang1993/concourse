package db

import (
	"database/sql"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
)

//go:generate counterfeiter . WorkerArtifactLifecycle

type WorkerArtifactLifecycle interface {
	RemoveExpiredArtifacts(logger lager.Logger) error
	RemoveOrphanedArtifacts(logger lager.Logger) error
}

type artifactLifecycle struct {
	conn Conn
}

func NewArtifactLifecycle(conn Conn) *artifactLifecycle {
	return &artifactLifecycle{
		conn: conn,
	}
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

	query, args, err := psql.Delete("worker_artifacts USING workers").
		Where(
			sq.Expr("worker_artifacts.worker_name = workers.name"),
		).
		Where(
			sq.Eq{
				"initialized":                  true,
				"build_id":                     sql.NullInt64{},
				"worker_resource_cache_id":     nil,
				"worker_task_cache_id":         nil,
				"worker_base_resource_type_id": nil,
				"worker_resource_certs_id":     nil,
			}).
		Where(sq.Or{
			sq.Expr("workers.state = 'running'::worker_state"),
			sq.Expr("workers.state = 'landing'::worker_state"),
			sq.Expr("workers.state = 'retiring'::worker_state"),
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
