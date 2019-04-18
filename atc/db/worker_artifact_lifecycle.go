package db

import (
	"database/sql"
	"time"

	"code.cloudfoundry.org/lager"
	sq "github.com/Masterminds/squirrel"
	"github.com/lib/pq"
)

const (
	ArtifactTypeResourceCache    = "resource-cache"
	ArtifactTypeTaskCache        = "task-cache"
	ArtifactTypeResourceCerts    = "resource-certs"
	ArtifactTypeBaseResourceType = "base-resource"
)

//go:generate counterfeiter . ArtifactProvider

type ArtifactProvider interface {
	CreateArtifact(name string) (WorkerArtifact, error)
}

//go:generate counterfeiter . WorkerArtifactLifecycle

type WorkerArtifactLifecycle interface {
	ArtifactProvider
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

func (lifecycle *artifactLifecycle) FindArtifactForResourceCache(logger lager.Logger, workerResourceCacheID int) (WorkerArtifact, error) {
	return lifecycle.findArtifact(sq.Eq{"worker_resource_cache_id": workerResourceCacheID})
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

func (lifecycle *artifactLifecycle) findArtifact(whereClause map[string]interface{}) (WorkerArtifact, error) {
	row := psql.Select("*").
		From("worker_artifacts").
		Where(whereClause).
		RunWith(lifecycle.conn).
		QueryRow()

	return scanArtifact(row, lifecycle.conn)
}

func scanArtifact(row sq.RowScanner, conn Conn) (WorkerArtifact, error) {
	var (
		sqID                       sql.NullInt64
		sqName                     sql.NullString
		sqCreatedAt                pq.NullTime
		sqInitialized              sql.NullBool
		sqBuildID                  sql.NullInt64
		sqResourceCacheID          sql.NullInt64
		sqWorkerBaseResourceTypeID sql.NullInt64
		sqWorkerTaskCacheID        sql.NullInt64
		sqWorkerResourceCertsID    sql.NullInt64
	)

	err := row.Scan(
		&sqID,
		&sqName,
		&sqBuildID,
		&sqCreatedAt,
		&sqResourceCacheID,
		&sqWorkerTaskCacheID,
		&sqWorkerResourceCertsID,
		&sqWorkerBaseResourceTypeID,
		&sqInitialized,
	)
	if err != nil {
		return nil, err
	}

	var id int
	if sqID.Valid {
		id = int(sqID.Int64)
	}

	var name string
	if sqName.Valid {
		name = sqName.String
	}

	var buildID int
	if sqBuildID.Valid {
		buildID = int(sqBuildID.Int64)
	}

	var createdAt time.Time
	if sqCreatedAt.Valid {
		createdAt = sqCreatedAt.Time
	}

	var initialized bool
	if sqInitialized.Valid {
		initialized = sqInitialized.Bool
	}
	var workerResourceCertsID int
	if sqWorkerResourceCertsID.Valid {
		workerResourceCertsID = int(sqWorkerResourceCertsID.Int64)
	}

	var workerTaskCacheID int
	if sqWorkerTaskCacheID.Valid {
		workerTaskCacheID = int(sqWorkerTaskCacheID.Int64)
	}

	var workerResourceCacheID int
	if sqResourceCacheID.Valid {
		workerResourceCacheID = int(sqResourceCacheID.Int64)
	}

	var workerBaseResourceTypeID int
	if sqWorkerBaseResourceTypeID.Valid {
		workerBaseResourceTypeID = int(sqWorkerBaseResourceTypeID.Int64)
	}

	return &artifact{
		conn,
		id,
		name,
		createdAt,
		initialized,
		buildID,
		workerResourceCacheID,
		workerBaseResourceTypeID,
		workerTaskCacheID,
		workerResourceCertsID,
	}, nil

}
func isTerminated(buildStatus sql.NullString) bool {
	return buildStatus.Valid && buildStatus.String != string(BuildStatusPending) && buildStatus.String != string(BuildStatusStarted)
}
