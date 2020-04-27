package db

import (
	"encoding/json"
	sq "github.com/Masterminds/squirrel"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/event"
	"github.com/lib/pq"
	"strconv"
)

//go:generate counterfeiter . BuildCreator

type BuildCreator interface {
	CreateStartedBuild(teamID int, pipelineID int, plan atc.Plan) (Build, error)
	CreateBuild(job Job) (Build, error)
	RerunBuild(job Job, buildToRerun Build) (Build, error)
	EnsurePendingBuildExists(job Job) error
}

type buildCreator struct {
	conn        Conn
	lockFactory lock.LockFactory

	eventProcessor EventProcessor
}

func NewBuildCreator(conn Conn, lockFactory lock.LockFactory, eventProcessor EventProcessor) BuildCreator {
	return &buildCreator{
		conn:        conn,
		lockFactory: lockFactory,

		eventProcessor: eventProcessor,
	}
}

func (b *buildCreator) CreateStartedBuild(teamID int, pipelineID int, plan atc.Plan) (Build, error) {
	tx, err := b.conn.Begin()
	if err != nil {
		return nil, err
	}
	defer Rollback(tx)
	metadata, err := json.Marshal(plan)
	if err != nil {
		return nil, err
	}
	encryptedPlan, nonce, err := b.conn.EncryptionStrategy().Encrypt(metadata)
	if err != nil {
		return nil, err
	}
	build := newEmptyBuild(b.conn, b.lockFactory)
	vals := map[string]interface{}{
		"name":         sq.Expr("nextval('one_off_name')"),
		"team_id":      teamID,
		"status":       BuildStatusStarted,
		"start_time":   sq.Expr("now()"),
		"schema":       schema,
		"private_plan": encryptedPlan,
		"public_plan":  plan.Public(),
		"nonce":        nonce,
	}
	if pipelineID > 0 {
		vals["pipeline_id"] = pipelineID
	}
	err = b.createBuild(tx, build, vals)
	if err != nil {
		return nil, err
	}
	err = tx.Commit()
	if err != nil {
		return nil, err
	}
	err = b.eventProcessor.Initialize(build)
	if err != nil {
		return nil, err
	}
	err = b.eventProcessor.Process(build, event.Status{
		Status: atc.StatusStarted,
		Time:   build.StartTime().Unix(),
	})
	if err != nil {
		return nil, err
	}
	if err = b.conn.Bus().Notify(buildStartedChannel()); err != nil {
		return nil, err
	}
	return build, nil
}

func (b *buildCreator) CreateBuild(j Job) (Build, error) {
	tx, err := b.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	buildName, err := getNewJobBuildName(tx, j)
	if err != nil {
		return nil, err
	}

	build := newEmptyBuild(b.conn, b.lockFactory)
	err = b.createBuild(tx, build, map[string]interface{}{
		"name":               buildName,
		"job_id":             j.ID(),
		"pipeline_id":        j.PipelineID(),
		"team_id":            j.TeamID(),
		"status":             BuildStatusPending,
		"manually_triggered": true,
	})
	if err != nil {
		return nil, err
	}

	latestNonRerunID, err := latestCompletedNonRerunBuild(tx, j.ID())
	if err != nil {
		return nil, err
	}

	err = updateNextBuildForJob(tx, j.ID(), latestNonRerunID)
	if err != nil {
		return nil, err
	}

	err = requestSchedule(tx, j.ID())
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	err = b.eventProcessor.Initialize(build)
	if err != nil {
		return nil, err
	}

	return build, nil
}

func (b *buildCreator) RerunBuild(job Job, buildToRerun Build) (Build, error) {
	for {
		rerunBuild, err := b.tryRerunBuild(job, buildToRerun)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code.Name() == pqUniqueViolationErrCode {
				continue
			}

			return nil, err
		}

		err = b.eventProcessor.Initialize(rerunBuild)
		if err != nil {
			return nil, err
		}

		return rerunBuild, nil
	}
}

func (b *buildCreator) tryRerunBuild(j Job, buildToRerun Build) (Build, error) {
	tx, err := b.conn.Begin()
	if err != nil {
		return nil, err
	}

	defer Rollback(tx)

	buildToRerunID := buildToRerun.ID()
	if buildToRerun.RerunOf() != 0 {
		buildToRerunID = buildToRerun.RerunOf()
	}

	rerunBuildName, rerunNumber, err := getNewRerunBuildName(tx, buildToRerunID)
	if err != nil {
		return nil, err
	}

	rerunBuild := newEmptyBuild(b.conn, b.lockFactory)
	err = b.createBuild(tx, rerunBuild, map[string]interface{}{
		"name":         rerunBuildName,
		"job_id":       j.ID(),
		"pipeline_id":  j.PipelineID(),
		"team_id":      j.TeamID(),
		"status":       BuildStatusPending,
		"rerun_of":     buildToRerunID,
		"rerun_number": rerunNumber,
	})
	if err != nil {
		return nil, err
	}

	latestNonRerunID, err := latestCompletedNonRerunBuild(tx, j.ID())
	if err != nil {
		return nil, err
	}

	err = updateNextBuildForJob(tx, j.ID(), latestNonRerunID)
	if err != nil {
		return nil, err
	}

	err = requestSchedule(tx, j.ID())
	if err != nil {
		return nil, err
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	return rerunBuild, nil
}

func (b *buildCreator) createBuild(tx Tx, build *build, vals map[string]interface{}) error {
	var buildID int

	buildVals := make(map[string]interface{})
	for name, value := range vals {
		buildVals[name] = value
	}

	buildVals["needs_v6_migration"] = false

	err := psql.Insert("builds").
		SetMap(buildVals).
		Suffix("RETURNING id").
		RunWith(tx).
		QueryRow().
		Scan(&buildID)
	if err != nil {
		return err
	}

	return scanBuild(build, buildsQuery.
		Where(sq.Eq{"b.id": buildID}).
		RunWith(tx).
		QueryRow(),
		b.conn.EncryptionStrategy(),
	)
}

func (b *buildCreator) EnsurePendingBuildExists(job Job) error {
	tx, err := b.conn.Begin()
	if err != nil {
		return err
	}

	defer Rollback(tx)

	buildName, err := getNewJobBuildName(tx, job)
	if err != nil {
		return err
	}

	rows, err := tx.Query(`
		INSERT INTO builds (name, job_id, pipeline_id, team_id, status, needs_v6_migration)
		SELECT $1, $2, $3, $4, 'pending', false
		WHERE NOT EXISTS
			(SELECT id FROM builds WHERE job_id = $2 AND status = 'pending')
		RETURNING id
	`, buildName, job.ID(), job.PipelineID(), job.TeamID())
	if err != nil {
		return err
	}

	defer Close(rows)

	if rows.Next() {
		var buildID int
		err := rows.Scan(&buildID)
		if err != nil {
			return err
		}

		err = rows.Close()
		if err != nil {
			return err
		}

		build := newEmptyBuild(b.conn, b.lockFactory)
		err = scanBuild(build, buildsQuery.
			Where(sq.Eq{"b.id": buildID}).
			RunWith(tx).
			QueryRow(),
			b.conn.EncryptionStrategy(),
		)
		if err != nil {
			return err
		}

		latestNonRerunID, err := latestCompletedNonRerunBuild(tx, job.ID())
		if err != nil {
			return err
		}

		err = updateNextBuildForJob(tx, job.ID(), latestNonRerunID)
		if err != nil {
			return err
		}

		err = tx.Commit()
		if err != nil {
			return err
		}

		return b.eventProcessor.Initialize(build)
	}

	return nil
}

func getNewJobBuildName(tx Tx, j Job) (string, error) {
	var buildName string
	err := psql.Update("jobs").
		Set("build_number_seq", sq.Expr("build_number_seq + 1")).
		Where(sq.Eq{
			"name":        j.Name(),
			"pipeline_id": j.PipelineID(),
		}).
		Suffix("RETURNING build_number_seq").
		RunWith(tx).
		QueryRow().
		Scan(&buildName)

	return buildName, err
}

func getNewRerunBuildName(tx Tx, buildID int) (string, int, error) {
	var rerunNum int
	var buildName string
	err := psql.Select("b.name", "( SELECT COUNT(id) FROM builds WHERE rerun_of = b.id )").
		From("builds b").
		Where(sq.Eq{
			"b.id": buildID,
		}).
		RunWith(tx).
		QueryRow().
		Scan(&buildName, &rerunNum)
	if err != nil {
		return "", 0, err
	}

	// increment the rerun number
	rerunNum++

	return buildName + "." + strconv.Itoa(rerunNum), rerunNum, err
}
