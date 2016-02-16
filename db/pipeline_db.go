package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/concourse/atc"
	"github.com/concourse/atc/config"
	"github.com/concourse/atc/db/algorithm"
	"github.com/pivotal-golang/lager"
)

//go:generate counterfeiter . PipelineDB

type PipelineDB interface {
	GetPipelineName() string
	ScopedName(string) string

	Pause() error
	Unpause() error
	IsPaused() (bool, error)

	Destroy() error

	GetConfig() (atc.Config, ConfigVersion, bool, error)

	LeaseScheduling(time.Duration) (Lease, bool, error)

	GetResource(resourceName string) (SavedResource, error)
	GetResourceVersions(resourceName string, page Page) ([]SavedVersionedResource, Pagination, bool, error)

	PauseResource(resourceName string) error
	UnpauseResource(resourceName string) error

	SaveResourceVersions(atc.ResourceConfig, []atc.Version) error
	GetLatestVersionedResource(resource SavedResource) (SavedVersionedResource, bool, error)
	GetLatestEnabledVersionedResource(resourceName string) (SavedVersionedResource, bool, error)
	EnableVersionedResource(versionedResourceID int) error
	DisableVersionedResource(versionedResourceID int) error
	SetResourceCheckError(resource SavedResource, err error) error
	LeaseResourceChecking(resource string, length time.Duration, immediate bool) (Lease, bool, error)

	GetJob(job string) (SavedJob, error)
	PauseJob(job string) error
	UnpauseJob(job string) error

	GetJobFinishedAndNextBuild(job string) (*Build, *Build, error)

	GetJobBuilds(job string, page Page) ([]Build, Pagination, error)
	GetAllJobBuilds(job string) ([]Build, error)

	GetJobBuild(job string, build string) (Build, bool, error)
	CreateJobBuild(job string) (Build, error)
	CreateJobBuildForCandidateInputs(job string) (Build, bool, error)

	UseInputsForBuild(buildID int, inputs []BuildInput) error

	LoadVersionsDB() (*algorithm.VersionsDB, error)
	GetLatestInputVersions(versions *algorithm.VersionsDB, job string, inputs []config.JobInput) ([]BuildInput, bool, error)
	GetJobBuildForInputs(job string, inputs []BuildInput) (Build, bool, error)
	GetNextPendingBuild(job string) (Build, bool, error)

	GetCurrentBuild(job string) (Build, bool, error)
	GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]Build, error)
	GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (Build, bool, error)

	ScheduleBuild(buildID int, job atc.JobConfig) (bool, error)
	SaveBuildInput(buildID int, input BuildInput) (SavedVersionedResource, error)
	SaveBuildOutput(buildID int, vr VersionedResource, explicit bool) (SavedVersionedResource, error)
	GetBuildsWithVersionAsInput(versionedResourceID int) ([]Build, error)
	GetBuildsWithVersionAsOutput(versionedResourceID int) ([]Build, error)

	GetDashboard() (Dashboard, atc.GroupConfigs, error)
}

type pipelineDB struct {
	logger lager.Logger

	conn Conn
	bus  *notificationsBus

	SavedPipeline

	versionsDB *algorithm.VersionsDB

	buildPrepHelper buildPreparationHelper
}

func (pdb *pipelineDB) GetPipelineName() string {
	return pdb.Name
}

func (pdb *pipelineDB) ScopedName(name string) string {
	return pdb.Name + ":" + name
}

func (pdb *pipelineDB) Unpause() error {
	_, err := pdb.conn.Exec(`
		UPDATE pipelines
		SET paused = false
		WHERE id = $1
	`, pdb.ID)
	return err
}

func (pdb *pipelineDB) Pause() error {
	_, err := pdb.conn.Exec(`
		UPDATE pipelines
		SET paused = true
		WHERE id = $1
	`, pdb.ID)
	if err != nil {
		return err
	}

	_, err = pdb.conn.Exec(`
			UPDATE build_preparation
			SET paused_pipeline='blocking',
			    paused_job='unknown',
					max_running_builds='unknown',
					inputs='{}'
			FROM build_preparation bp, builds b, jobs j
			WHERE bp.build_id = b.id AND b.job_id = j.id
				AND j.pipeline_id = $1 AND b.status = 'pending'
		`, pdb.ID)
	return err
}

func scanIDs(rows *sql.Rows) ([]string, error) {
	defer rows.Close()

	ids := []string{}
	for rows.Next() {
		var id string
		err := rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

func (pdb *pipelineDB) Destroy() error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	_, err = tx.Exec(fmt.Sprintf(`
		DROP TABLE pipeline_build_events_%d
	`, pdb.ID))
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM pipelines WHERE id = $1;
	`, pdb.ID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (pdb *pipelineDB) GetConfig() (atc.Config, ConfigVersion, bool, error) {
	var configBlob []byte
	var version int

	err := pdb.conn.QueryRow(`
			SELECT config, version
			FROM pipelines
			WHERE id = $1
		`, pdb.ID).Scan(&configBlob, &version)
	if err != nil {
		if err == sql.ErrNoRows {
			return atc.Config{}, 0, false, nil
		}

		return atc.Config{}, 0, false, err
	}

	var config atc.Config
	err = json.Unmarshal(configBlob, &config)
	if err != nil {
		return atc.Config{}, 0, false, err
	}

	return config, ConfigVersion(version), true, nil
}

func (pdb *pipelineDB) GetResource(resourceName string) (SavedResource, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedResource{}, err
	}

	defer tx.Rollback()

	resource, err := pdb.getResource(tx, resourceName)
	if err != nil {
		return SavedResource{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedResource{}, err
	}

	return resource, nil
}

func (pdb *pipelineDB) LeaseResourceChecking(resourceName string, interval time.Duration, immediate bool) (Lease, bool, error) {
	logger := pdb.logger.Session("lease", lager.Data{
		"resource": resourceName,
	})

	lease := &lease{
		conn:   pdb.conn,
		logger: logger,
		attemptSignFunc: func(tx Tx) (sql.Result, error) {
			params := []interface{}{resourceName, pdb.ID}

			condition := ""
			if immediate {
				condition = "NOT checking"
			} else {
				condition = "now() - last_checked > ($3 || ' SECONDS')::INTERVAL"
				params = append(params, interval.Seconds())
			}

			return tx.Exec(`
				UPDATE resources
				SET last_checked = now(), checking = true
				WHERE name = $1
					AND pipeline_id = $2
					AND `+condition, params...)
		},
		heartbeatFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE resources
				SET last_checked = now()
				WHERE name = $1
					AND pipeline_id = $2
			`, resourceName, pdb.ID)
		},
		breakFunc: func() {
			_, err := pdb.conn.Exec(`
				UPDATE resources
				SET checking = false
				WHERE name = $1
				  AND pipeline_id = $2
			`, resourceName, pdb.ID)
			if err != nil {
				logger.Error("failed-to-reset-checking-state", err)
			}
		},
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}

func (pdb *pipelineDB) LeaseScheduling(interval time.Duration) (Lease, bool, error) {
	lease := &lease{
		conn: pdb.conn,
		logger: pdb.logger.Session("lease", lager.Data{
			"pipeline": pdb.Name,
		}),
		attemptSignFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE pipelines
				SET last_scheduled = now()
				WHERE id = $1
					AND now() - last_scheduled > ($2 || ' SECONDS')::INTERVAL
			`, pdb.ID, interval.Seconds())
		},
		heartbeatFunc: func(tx Tx) (sql.Result, error) {
			return tx.Exec(`
				UPDATE pipelines
				SET last_scheduled = now()
				WHERE id = $1
			`, pdb.ID)
		},
	}

	renewed, err := lease.AttemptSign(interval)
	if err != nil {
		return nil, false, err
	}

	if !renewed {
		return nil, renewed, nil
	}

	lease.KeepSigned(interval)

	return lease, true, nil
}

func (pdb *pipelineDB) GetResourceVersions(resourceName string, page Page) ([]SavedVersionedResource, Pagination, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return []SavedVersionedResource{}, Pagination{}, false, err
	}

	defer tx.Rollback()

	dbResource, err := pdb.getResource(tx, resourceName)
	if err != nil {
		if err == sql.ErrNoRows {
			return []SavedVersionedResource{}, Pagination{}, false, nil
		}
		return []SavedVersionedResource{}, Pagination{}, false, err
	}

	query := `
		SELECT v.id, v.enabled, v.type, v.version, v.metadata, r.name
		FROM versioned_resources v
		INNER JOIN resources r ON v.resource_id = r.id
		WHERE v.resource_id = $1
	`

	var rows *sql.Rows
	if page.Since == 0 && page.Until == 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			%s
			ORDER BY v.id DESC
			LIMIT $2
		`, query), dbResource.ID, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else if page.Until != 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			SELECT sub.*
				FROM (
						%s
					AND v.id > $2
				ORDER BY v.id ASC
				LIMIT $3
			) sub
			ORDER BY sub.id DESC
		`, query), dbResource.ID, page.Until, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	} else {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			%s
				AND v.id < $2
			ORDER BY v.id DESC
			LIMIT $3
		`, query), dbResource.ID, page.Since, page.Limit)
		if err != nil {
			return nil, Pagination{}, false, err
		}
	}

	defer rows.Close()

	savedVersionedResources := make([]SavedVersionedResource, 0)
	for rows.Next() {
		var savedVersionedResource SavedVersionedResource

		var versionString, metadataString string

		err := rows.Scan(
			&savedVersionedResource.ID,
			&savedVersionedResource.Enabled,
			&savedVersionedResource.Type,
			&versionString,
			&metadataString,
			&savedVersionedResource.Resource,
		)
		if err != nil {
			return nil, Pagination{}, false, err
		}

		err = json.Unmarshal([]byte(versionString), &savedVersionedResource.Version)
		if err != nil {
			return nil, Pagination{}, false, err
		}

		err = json.Unmarshal([]byte(metadataString), &savedVersionedResource.Metadata)
		if err != nil {
			return nil, Pagination{}, false, err
		}

		savedVersionedResource.PipelineName = pdb.GetPipelineName()

		savedVersionedResources = append(savedVersionedResources, savedVersionedResource)
	}

	if len(savedVersionedResources) == 0 {
		return []SavedVersionedResource{}, Pagination{}, true, nil
	}

	var minID int
	var maxID int

	err = pdb.conn.QueryRow(`
		SELECT COALESCE(MAX(v.id), 0) as maxID,
			COALESCE(MIN(v.id), 0) as minID
		FROM versioned_resources v
		WHERE v.resource_id = $1
	`, dbResource.ID).Scan(&maxID, &minID)
	if err != nil {
		return nil, Pagination{}, false, err
	}

	firstSavedVersionedResource := savedVersionedResources[0]
	lastSavedVersionedResource := savedVersionedResources[len(savedVersionedResources)-1]

	var pagination Pagination

	if firstSavedVersionedResource.ID < maxID {
		pagination.Previous = &Page{
			Until: firstSavedVersionedResource.ID,
			Limit: page.Limit,
		}
	}

	if lastSavedVersionedResource.ID > minID {
		pagination.Next = &Page{
			Since: lastSavedVersionedResource.ID,
			Limit: page.Limit,
		}
	}

	return savedVersionedResources, pagination, true, nil
}

func (pdb *pipelineDB) getResource(tx Tx, name string) (SavedResource, error) {
	var checkErr sql.NullString
	var resource SavedResource

	err := tx.QueryRow(`
			SELECT id, name, check_error, paused
			FROM resources
			WHERE name = $1
				AND pipeline_id = $2
		`, name, pdb.ID).Scan(&resource.ID, &resource.Name, &checkErr, &resource.Paused)
	if err != nil {
		return SavedResource{}, err
	}

	if checkErr.Valid {
		resource.CheckError = errors.New(checkErr.String)
	}

	resource.PipelineName = pdb.Name

	return resource, nil
}

func (pdb *pipelineDB) PauseResource(resource string) error {
	return pdb.updatePaused(resource, true)
}

func (pdb *pipelineDB) UnpauseResource(resource string) error {
	return pdb.updatePaused(resource, false)
}

func (pdb *pipelineDB) updatePaused(resource string, pause bool) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	result, err := tx.Exec(`
		UPDATE resources
		SET paused = $1
		WHERE name = $2
			AND pipeline_id = $3
	`, pause, resource, pdb.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return nonOneRowAffectedError{rowsAffected}
	}

	return tx.Commit()
}

func (pdb *pipelineDB) SaveResourceVersions(config atc.ResourceConfig, versions []atc.Version) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	for _, version := range versions {
		_, err := pdb.saveVersionedResource(tx, VersionedResource{
			Resource: config.Name,
			Type:     config.Type,
			Version:  Version(version),
		})
		if err != nil {
			return err
		}
	}

	err = tx.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (pdb *pipelineDB) DisableVersionedResource(versionedResourceID int) error {
	return pdb.toggleVersionedResource(versionedResourceID, false)
}

func (pdb *pipelineDB) EnableVersionedResource(versionedResourceID int) error {
	return pdb.toggleVersionedResource(versionedResourceID, true)
}

func (pdb *pipelineDB) toggleVersionedResource(versionedResourceID int, enable bool) error {
	rows, err := pdb.conn.Exec(`
		UPDATE versioned_resources
		SET enabled = $1, modified_time = now()
		WHERE id = $2
	`, enable, versionedResourceID)
	if err != nil {
		return err
	}

	rowsAffected, err := rows.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return nonOneRowAffectedError{rowsAffected}
	}

	return nil
}

func (pdb *pipelineDB) GetLatestEnabledVersionedResource(resourceName string) (SavedVersionedResource, bool, error) {
	var versionBytes, metadataBytes string

	svr := SavedVersionedResource{
		VersionedResource: VersionedResource{
			Resource: resourceName,
		},
	}

	err := pdb.conn.QueryRow(`
		SELECT v.id, v.enabled, v.type, v.version, v.metadata, v.modified_time
		FROM versioned_resources v, resources r
		WHERE v.resource_id = r.id
			AND r.name = $1
			AND enabled = true
			AND r.pipeline_id = $2
		ORDER BY id DESC
		LIMIT 1
	`, resourceName, pdb.ID).Scan(&svr.ID, &svr.Enabled, &svr.Type, &versionBytes, &metadataBytes, &svr.ModifiedTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return SavedVersionedResource{}, false, nil
		}

		return SavedVersionedResource{}, false, err
	}

	err = json.Unmarshal([]byte(versionBytes), &svr.Version)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	err = json.Unmarshal([]byte(metadataBytes), &svr.Metadata)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	return svr, true, nil
}

func (pdb *pipelineDB) GetLatestVersionedResource(resource SavedResource) (SavedVersionedResource, bool, error) {
	var versionBytes, metadataBytes string

	svr := SavedVersionedResource{
		VersionedResource: VersionedResource{
			Resource: resource.Name,
		},
	}

	err := pdb.conn.QueryRow(`
		SELECT id, enabled, type, version, metadata, modified_time
		FROM versioned_resources
		WHERE resource_id = $1
		ORDER BY id DESC
		LIMIT 1
	`, resource.ID).Scan(&svr.ID, &svr.Enabled, &svr.Type, &versionBytes, &metadataBytes, &svr.ModifiedTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return SavedVersionedResource{}, false, nil
		}

		return SavedVersionedResource{}, false, err
	}

	err = json.Unmarshal([]byte(versionBytes), &svr.Version)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	err = json.Unmarshal([]byte(metadataBytes), &svr.Metadata)
	if err != nil {
		return SavedVersionedResource{}, false, err
	}

	return svr, true, nil
}

func (pdb *pipelineDB) SetResourceCheckError(resource SavedResource, cause error) error {
	var err error

	if cause == nil {
		_, err = pdb.conn.Exec(`
			UPDATE resources
			SET check_error = NULL
			WHERE id = $1
			`, resource.ID)
	} else {
		_, err = pdb.conn.Exec(`
			UPDATE resources
			SET check_error = $2
			WHERE id = $1
		`, resource.ID, cause.Error())
	}

	return err
}

func (pdb *pipelineDB) saveVersionedResource(tx Tx, vr VersionedResource) (SavedVersionedResource, error) {
	savedResource, err := pdb.getResource(tx, vr.Resource)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	versionJSON, err := json.Marshal(vr.Version)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	metadataJSON, err := json.Marshal(vr.Metadata)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	var id int
	var enabled bool
	var modified_time time.Time

	_, err = tx.Exec(`
		INSERT INTO versioned_resources (resource_id, type, version, metadata, modified_time)
		SELECT $1, $2, $3, $4, now()
		WHERE NOT EXISTS (
			SELECT 1
			FROM versioned_resources
			WHERE resource_id = $1
			AND type = $2
			AND version = $3
		)
	`, savedResource.ID, vr.Type, string(versionJSON), string(metadataJSON))

	err = swallowUniqueViolation(err)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	var savedMetadata string

	// separate from above, as it conditionally inserts (can't use RETURNING)
	if len(vr.Metadata) > 0 {
		err = tx.QueryRow(`
			UPDATE versioned_resources
			SET metadata = $4, modified_time = now()
			WHERE resource_id = $1
			AND type = $2
			AND version = $3
			RETURNING id, enabled, metadata, modified_time
		`, savedResource.ID, vr.Type, string(versionJSON), string(metadataJSON)).Scan(&id, &enabled, &savedMetadata, &modified_time)
	} else {
		err = tx.QueryRow(`
			SELECT id, enabled, metadata, modified_time
			FROM versioned_resources
			WHERE resource_id = $1
			AND type = $2
			AND version = $3
		`, savedResource.ID, vr.Type, string(versionJSON)).Scan(&id, &enabled, &savedMetadata, &modified_time)
	}
	if err != nil {
		return SavedVersionedResource{}, err
	}

	err = json.Unmarshal([]byte(savedMetadata), &vr.Metadata)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return SavedVersionedResource{
		ID:           id,
		Enabled:      enabled,
		ModifiedTime: modified_time,

		VersionedResource: vr,
	}, nil
}

func (pdb *pipelineDB) GetJob(jobName string) (SavedJob, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedJob{}, err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, jobName)
	if err != nil {
		return SavedJob{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedJob{}, err
	}

	return dbJob, nil
}

func (pdb *pipelineDB) GetJobBuild(job string, name string) (Build, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, false, err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return Build{}, false, err
	}

	build, found, err := scanBuild(tx.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.job_id = $1
		AND b.name = $2
	`, dbJob.ID, name))
	if err != nil {
		return Build{}, false, err
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, false, err
	}

	return build, found, nil
}

func (pdb *pipelineDB) CreateJobBuildForCandidateInputs(jobName string) (Build, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, false, err
	}

	defer tx.Rollback()

	var x int
	err = tx.QueryRow(`
		SELECT 1
		FROM builds b
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE j.name = $1
			AND p.id = $2
			AND b.inputs_determined = false
			AND b.status IN ('pending', 'started')
	`, jobName, pdb.ID).Scan(&x)

	if err == sql.ErrNoRows {
		build, err := pdb.createJobBuild(jobName, tx)
		if err != nil {
			return Build{}, false, err
		}

		err = tx.Commit()
		if err != nil {
			return Build{}, false, err
		}

		return build, true, nil
	} else if err != nil {
		return Build{}, false, err
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, false, err
	}

	return Build{}, false, nil
}

func (pdb *pipelineDB) UseInputsForBuild(buildID int, inputs []BuildInput) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	for _, input := range inputs {
		_, err := pdb.saveBuildInput(tx, buildID, input)
		if err != nil {
			return err
		}
	}

	result, err := tx.Exec(`
		UPDATE builds b
		SET inputs_determined = true
		WHERE b.id = $1
	`, buildID)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows != 1 {
		return errors.New("multiple rows affected but expected only one when determining inputs")
	}

	return tx.Commit()
}

func (pdb *pipelineDB) CreateJobBuild(jobName string) (Build, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, err
	}

	defer tx.Rollback()

	build, err := pdb.createJobBuild(jobName, tx)
	if err != nil {
		return Build{}, err
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (pdb *pipelineDB) createJobBuild(jobName string, tx Tx) (Build, error) {
	dbJob, err := pdb.getJob(tx, jobName)
	if err != nil {
		return Build{}, err
	}

	var name string

	err = tx.QueryRow(`
		UPDATE jobs
		SET build_number_seq = build_number_seq + 1
		WHERE id = $1
		RETURNING build_number_seq
	`, dbJob.ID).Scan(&name)
	if err != nil {
		return Build{}, err
	}

	// We had to resort to sub-selects here because you can't paramaterize a
	// RETURNING statement in lib/pq... sorry

	build, _, err := scanBuild(tx.QueryRow(`
		INSERT INTO builds (name, job_id, status)
		VALUES ($1, $2, 'pending')
		RETURNING `+buildColumns+`,
			(
				SELECT j.name
				FROM jobs j
				WHERE j.id = job_id
			),
			(
				SELECT j.pipeline_id
				FROM jobs j
				WHERE j.id = job_id
			),
			(
				SELECT p.name
				FROM jobs j
				INNER JOIN pipelines p ON j.pipeline_id = p.id
				WHERE j.id = job_id
			)
	`, name, dbJob.ID))
	if err != nil {
		return Build{}, err
	}

	_, err = tx.Exec(fmt.Sprintf(`
		CREATE SEQUENCE %s MINVALUE 0
	`, buildEventSeq(build.ID)))
	if err != nil {
		return Build{}, err
	}

	err = pdb.buildPrepHelper.CreateBuildPreparation(tx, build.ID)
	if err != nil {
		return Build{}, err
	}

	return build, nil
}

func (pdb *pipelineDB) GetBuildsWithVersionAsInput(versionedResourceID int) ([]Build, error) {
	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN build_inputs bi ON bi.build_id = b.id
		WHERE bi.versioned_resource_id = $1
	`, versionedResourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	builds := []Build{}
	for rows.Next() {
		build, _, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		builds = append(builds, build)
	}

	return builds, err
}

func (pdb *pipelineDB) GetBuildsWithVersionAsOutput(versionedResourceID int) ([]Build, error) {
	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN build_outputs bo ON bo.build_id = b.id
		WHERE bo.versioned_resource_id = $1
	`, versionedResourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	builds := []Build{}
	for rows.Next() {
		build, _, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		builds = append(builds, build)
	}

	return builds, err
}

func (pdb *pipelineDB) SaveBuildInput(buildID int, input BuildInput) (SavedVersionedResource, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	defer tx.Rollback()

	svr, err := pdb.saveBuildInput(tx, buildID, input)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return svr, nil
}

func (pdb *pipelineDB) saveBuildInput(tx Tx, buildID int, input BuildInput) (SavedVersionedResource, error) {
	svr, err := pdb.saveVersionedResource(tx, input.VersionedResource)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	_, err = tx.Exec(`
		INSERT INTO build_inputs (build_id, versioned_resource_id, name)
		SELECT $1, $2, $3
		WHERE NOT EXISTS (
			SELECT 1
			FROM build_inputs
			WHERE build_id = $1
			AND versioned_resource_id = $2
			AND name = $3
		)
	`, buildID, svr.ID, input.Name)

	err = swallowUniqueViolation(err)

	if err != nil {
		return SavedVersionedResource{}, err
	}

	return svr, nil
}

func (pdb *pipelineDB) SaveBuildOutput(buildID int, vr VersionedResource, explicit bool) (SavedVersionedResource, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	defer tx.Rollback()

	svr, err := pdb.saveVersionedResource(tx, vr)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	_, err = tx.Exec(`
		INSERT INTO build_outputs (build_id, versioned_resource_id, explicit)
		VALUES ($1, $2, $3)
	`, buildID, svr.ID, explicit)
	if err != nil {
		return SavedVersionedResource{}, err
	}

	err = tx.Commit()
	if err != nil {
		return SavedVersionedResource{}, err
	}

	return svr, nil
}

func (pdb *pipelineDB) GetJobBuildForInputs(job string, inputs []BuildInput) (Build, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, false, err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return Build{}, false, err
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, false, err
	}

	from := []string{"builds b"}
	from = append(from, "jobs j")
	from = append(from, "pipelines p")
	conditions := []string{"job_id = $1"}
	conditions = append(conditions, "b.job_id = j.id")
	conditions = append(conditions, "j.pipeline_id = p.id")
	params := []interface{}{dbJob.ID}

	for i, input := range inputs {
		vr := input.VersionedResource
		dbResource, err := pdb.GetResource(vr.Resource)
		if err != nil {
			return Build{}, false, err
		}

		versionBytes, err := json.Marshal(vr.Version)
		if err != nil {
			return Build{}, false, err
		}

		var id int

		err = pdb.conn.QueryRow(`
			SELECT id
			FROM versioned_resources
			WHERE resource_id = $1
			AND type = $2
			AND version = $3
		`, dbResource.ID, vr.Type, string(versionBytes)).Scan(&id)
		if err == sql.ErrNoRows {
			return Build{}, false, nil
		}

		if err != nil {
			return Build{}, false, err
		}

		from = append(from, fmt.Sprintf("build_inputs i%d", i+1))
		params = append(params, id, input.Name)

		conditions = append(conditions,
			fmt.Sprintf("i%d.build_id = b.id", i+1),
			fmt.Sprintf("i%d.versioned_resource_id = $%d", i+1, len(params)-1),
			fmt.Sprintf("i%d.name = $%d", i+1, len(params)),
		)
	}

	return scanBuild(pdb.conn.QueryRow(fmt.Sprintf(`
		SELECT `+qualifiedBuildColumns+`
		FROM %s
		WHERE %s
		`,
		strings.Join(from, ", "),
		strings.Join(conditions, "\nAND ")),
		params...,
	))
}

func (pdb *pipelineDB) GetNextPendingBuild(job string) (Build, bool, error) {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return Build{}, false, err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return Build{}, false, err
	}

	err = tx.Commit()
	if err != nil {
		return Build{}, false, err
	}

	return scanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.job_id = $1
		AND b.status = 'pending'
		ORDER BY b.id ASC
		LIMIT 1
	`, dbJob.ID))
}

func (pdb *pipelineDB) updateSerialGroupsForJob(jobName string, serialGroups []string) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, jobName)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		DELETE FROM jobs_serial_groups
		WHERE job_id = $1
	`, dbJob.ID)
	if err != nil {
		return err
	}

	for _, serialGroup := range serialGroups {
		_, err = tx.Exec(`
			INSERT INTO jobs_serial_groups (job_id, serial_group)
			VALUES ($1, $2)
		`, dbJob.ID, serialGroup)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (pdb *pipelineDB) GetNextPendingBuildBySerialGroup(jobName string, serialGroups []string) (Build, bool, error) {
	pdb.updateSerialGroupsForJob(jobName, serialGroups)

	serialGroupNames := []interface{}{}
	refs := []string{}
	serialGroupNames = append(serialGroupNames, pdb.ID)
	for i, serialGroup := range serialGroups {
		serialGroupNames = append(serialGroupNames, serialGroup)
		refs = append(refs, fmt.Sprintf("$%d", i+2))
	}

	return scanBuild(pdb.conn.QueryRow(`
		SELECT DISTINCT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN jobs_serial_groups jsg ON j.id = jsg.job_id
				AND jsg.serial_group IN (`+strings.Join(refs, ",")+`)
		WHERE b.status = 'pending'
			AND j.pipeline_id = $1
		ORDER BY b.id ASC
		LIMIT 1
	`, serialGroupNames...))
}

func (pdb *pipelineDB) GetRunningBuildsBySerialGroup(jobName string, serialGroups []string) ([]Build, error) {
	pdb.updateSerialGroupsForJob(jobName, serialGroups)

	serialGroupNames := []interface{}{}
	refs := []string{}
	serialGroupNames = append(serialGroupNames, pdb.ID)
	for i, serialGroup := range serialGroups {
		serialGroupNames = append(serialGroupNames, serialGroup)
		refs = append(refs, fmt.Sprintf("$%d", i+2))
	}

	rows, err := pdb.conn.Query(`
		SELECT DISTINCT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		INNER JOIN jobs_serial_groups jsg ON j.id = jsg.job_id
				AND jsg.serial_group IN (`+strings.Join(refs, ",")+`)
		WHERE (
				b.status = 'started'
				OR
				(b.scheduled = true AND b.status = 'pending')
			)
			AND j.pipeline_id = $1
	`, serialGroupNames...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, _, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (pdb *pipelineDB) getBuild(buildID int) (Build, bool, error) {
	return scanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE b.id = $1
	`, buildID))
}

func (pdb *pipelineDB) updateBuildPreparation(prep BuildPreparation) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	err = pdb.buildPrepHelper.UpdateBuildPreparation(tx, prep)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (pdb *pipelineDB) ScheduleBuild(buildID int, jobConfig atc.JobConfig) (bool, error) {
	build, found, err := pdb.getBuild(buildID)
	if err != nil {
		return false, err
	}

	if !found {
		pdb.logger.Debug("build-deleted-while-scheduling", lager.Data{
			"buildID": buildID,
		})
		return false, nil
	}

	buildPrep, found, err := pdb.buildPrepHelper.GetBuildPreparation(pdb.conn, build.ID)
	if err != nil {
		return false, err
	}
	if !found {
		pdb.logger.Debug("unable-to-find-build-preparation-for-build", lager.Data{
			"buildID": buildID,
		})
		return false, nil
	}

	pipelinePaused, err := pdb.IsPaused()
	if err != nil {
		pdb.logger.Error("build-did-not-schedule", err, lager.Data{
			"reason":  "unexpected error",
			"buildID": buildID,
		})
		return false, err
	}

	if pipelinePaused {
		pdb.logger.Debug("build-did-not-schedule", lager.Data{
			"reason":  "pipeline-paused",
			"buildID": buildID,
		})

		buildPrep.PausedPipeline = BuildPreparationStatusBlocking
		err = pdb.updateBuildPreparation(buildPrep)

		return false, err
	}

	buildPrep.PausedPipeline = BuildPreparationStatusNotBlocking
	err = pdb.updateBuildPreparation(buildPrep)
	if err != nil {
		return false, err
	}

	// The function needs to be idempotent, that's why this isn't in CanBuildBeScheduled
	if build.Scheduled {
		buildPrep.PausedJob = BuildPreparationStatusNotBlocking
		buildPrep.MaxRunningBuilds = BuildPreparationStatusNotBlocking
		err = pdb.updateBuildPreparation(buildPrep)
		if err != nil {
			return false, err
		}

		return true, nil
	}

	jobService, err := NewJobService(jobConfig, pdb)
	if err != nil {
		return false, err
	}

	canBuildBeScheduled, reason, err := jobService.CanBuildBeScheduled(build)
	if err != nil {
		return false, err
	}

	if canBuildBeScheduled {
		buildPrep.PausedJob = BuildPreparationStatusNotBlocking
		buildPrep.MaxRunningBuilds = BuildPreparationStatusNotBlocking
		err = pdb.updateBuildPreparation(buildPrep)
		if err != nil {
			return false, err
		}

		updated, err := pdb.updateBuildToScheduled(buildID)
		if err != nil {
			return false, err
		}

		return updated, nil
	} else {
		pdb.logger.Debug("build-did-not-schedule", lager.Data{
			"reason":  reason,
			"buildID": buildID,
		})

		switch reason {
		case "job-paused":
			buildPrep.PausedJob = BuildPreparationStatusBlocking
			err = pdb.updateBuildPreparation(buildPrep)
			if err != nil {
				return false, err
			}
		case "max-in-flight-reached":
			buildPrep.MaxRunningBuilds = BuildPreparationStatusBlocking
			err = pdb.updateBuildPreparation(buildPrep)
			if err != nil {
				return false, err
			}
		}

		return false, nil
	}
}

func (pdb *pipelineDB) IsPaused() (bool, error) {
	var paused bool

	err := pdb.conn.QueryRow(`
		SELECT paused
		FROM pipelines
		WHERE id = $1
	`, pdb.ID).Scan(&paused)

	if err != nil {
		return false, err
	}

	return paused, nil
}

func (pdb *pipelineDB) updateBuildToScheduled(buildID int) (bool, error) {
	result, err := pdb.conn.Exec(`
			UPDATE builds
			SET scheduled = true
			WHERE id = $1
	`, buildID)
	if err != nil {
		return false, err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	return rows == 1, nil
}

func (pdb *pipelineDB) GetCurrentBuild(job string) (Build, bool, error) {
	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE j.name = $1
		AND j.pipeline_id = $2
		AND b.status != 'pending'
		ORDER BY b.id DESC
		LIMIT 1
	`, job, pdb.ID)
	if err != nil {
		return Build{}, false, err
	}

	defer rows.Close()

	if rows.Next() {
		return scanBuild(rows)
	}

	pendingRows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE j.name = $1
		AND j.pipeline_id = $2
		AND b.status = 'pending'
		ORDER BY b.id ASC
		LIMIT 1
		`, job, pdb.ID)
	if err != nil {
		return Build{}, false, err
	}

	defer pendingRows.Close()

	if pendingRows.Next() {
		return scanBuild(pendingRows)
	}

	return Build{}, false, nil
}

func (pdb *pipelineDB) getLastestModifiedTime() (time.Time, error) {
	var max_modified_time time.Time

	err := pdb.conn.QueryRow(`
	SELECT
		CASE
			WHEN bo_max > vr_max THEN bo_max
			ELSE vr_max
		END
	FROM
		(
			SELECT COALESCE(MAX(bo.modified_time), 'epoch') as bo_max
			FROM build_outputs bo
			LEFT OUTER JOIN versioned_resources v ON v.id = bo.versioned_resource_id
			LEFT OUTER JOIN resources r ON r.id = v.resource_id
			WHERE r.pipeline_id = $1
		) bo,
		(
			SELECT COALESCE(MAX(vr.modified_time), 'epoch') as vr_max
			FROM versioned_resources vr
			LEFT OUTER JOIN resources r ON r.id = vr.resource_id
			WHERE r.pipeline_id = $1
		) vr
	`, pdb.ID).Scan(&max_modified_time)

	return max_modified_time, err
}

func (pdb *pipelineDB) LoadVersionsDB() (*algorithm.VersionsDB, error) {
	latestModifiedTime, err := pdb.getLastestModifiedTime()
	if err != nil {
		return nil, err
	}

	if pdb.versionsDB != nil && pdb.versionsDB.CachedAt.Equal(latestModifiedTime) {
		return pdb.versionsDB, nil
	}

	db := &algorithm.VersionsDB{
		BuildOutputs:     []algorithm.BuildOutput{},
		ResourceVersions: []algorithm.ResourceVersion{},
		JobIDs:           map[string]int{},
		ResourceIDs:      map[string]int{},
		CachedAt:         latestModifiedTime,
	}

	rows, err := pdb.conn.Query(`
    SELECT v.id, r.id, o.build_id, j.id
    FROM build_outputs o, builds b, versioned_resources v, jobs j, resources r
    WHERE v.id = o.versioned_resource_id
    AND b.id = o.build_id
    AND j.id = b.job_id
    AND r.id = v.resource_id
    AND v.enabled
		AND b.status = 'succeeded'
		AND r.pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var output algorithm.BuildOutput
		err := rows.Scan(&output.VersionID, &output.ResourceID, &output.BuildID, &output.JobID)
		if err != nil {
			return nil, err
		}

		db.BuildOutputs = append(db.BuildOutputs, output)
	}

	rows, err = pdb.conn.Query(`
    SELECT v.id, r.id
    FROM versioned_resources v, resources r
    WHERE r.id = v.resource_id
    AND v.enabled
		AND r.pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var output algorithm.ResourceVersion
		err := rows.Scan(&output.VersionID, &output.ResourceID)
		if err != nil {
			return nil, err
		}

		db.ResourceVersions = append(db.ResourceVersions, output)
	}

	rows, err = pdb.conn.Query(`
    SELECT j.name, j.id
    FROM jobs j
    WHERE j.pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var name string
		var id int
		err := rows.Scan(&name, &id)
		if err != nil {
			return nil, err
		}

		db.JobIDs[name] = id
	}

	rows, err = pdb.conn.Query(`
    SELECT r.name, r.id
    FROM resources r
    WHERE r.pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var name string
		var id int
		err := rows.Scan(&name, &id)
		if err != nil {
			return nil, err
		}

		db.ResourceIDs[name] = id
	}

	pdb.versionsDB = db

	return db, nil
}

func (pdb *pipelineDB) GetLatestInputVersions(db *algorithm.VersionsDB, jobName string, inputs []config.JobInput) ([]BuildInput, bool, error) {
	if len(inputs) == 0 {
		return []BuildInput{}, true, nil
	}

	var inputConfigs algorithm.InputConfigs

	for _, input := range inputs {
		jobs := algorithm.JobSet{}
		for _, jobName := range input.Passed {
			jobs[db.JobIDs[jobName]] = struct{}{}
		}

		inputConfigs = append(inputConfigs, algorithm.InputConfig{
			Name:       input.Name,
			ResourceID: db.ResourceIDs[input.Resource],
			Passed:     jobs,
		})
	}

	resolved, ok := inputConfigs.Resolve(db)
	if !ok {
		return nil, false, nil
	}

	var buildInputs []BuildInput

	for name, id := range resolved {
		svr := SavedVersionedResource{
			ID:      id,
			Enabled: true, // this is inherent with the following query
		}

		var version, metadata string

		err := pdb.conn.QueryRow(`
			SELECT r.name, vr.type, vr.version, vr.metadata
			FROM versioned_resources vr, resources r
			WHERE vr.id = $1
				AND vr.resource_id = r.id
		`, id).Scan(&svr.Resource, &svr.Type, &version, &metadata)
		if err != nil {
			return nil, false, err
		}

		err = json.Unmarshal([]byte(version), &svr.Version)
		if err != nil {
			return nil, false, err
		}

		err = json.Unmarshal([]byte(metadata), &svr.Metadata)
		if err != nil {
			return nil, false, err
		}

		buildInputs = append(buildInputs, BuildInput{
			Name:              name,
			VersionedResource: svr.VersionedResource,
		})
	}

	return buildInputs, true, nil
}

func (pdb *pipelineDB) PauseJob(job string) error {
	return pdb.updatePausedJob(job, true)
}

func (pdb *pipelineDB) UnpauseJob(job string) error {
	return pdb.updatePausedJob(job, false)
}

func (pdb *pipelineDB) updatePausedJob(job string, pause bool) error {
	tx, err := pdb.conn.Begin()
	if err != nil {
		return err
	}

	defer tx.Rollback()

	dbJob, err := pdb.getJob(tx, job)
	if err != nil {
		return err
	}

	result, err := tx.Exec(`
		UPDATE jobs
		SET paused = $1
		WHERE id = $2
	`, pause, dbJob.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected != 1 {
		return nonOneRowAffectedError{rowsAffected}
	}

	if pause {
		result, err = tx.Exec(`
    UPDATE build_preparation
		SET paused_job = 'blocking',
		    max_running_builds = 'unknown',
				inputs = '{}'
		FROM build_preparation bp, builds b
		WHERE b.id = bp.build_id
		  AND b.job_id = $1
	`, dbJob.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (pdb *pipelineDB) GetJobBuilds(jobName string, page Page) ([]Build, Pagination, error) {
	var (
		err        error
		maxID      int
		minID      int
		firstBuild Build
		lastBuild  Build
		pagination Pagination

		rows *sql.Rows
	)

	query := fmt.Sprintf(`
		SELECT ` + qualifiedBuildColumns + `
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE j.name = $1
			AND j.pipeline_id = $2
	`)

	if page.Since == 0 && page.Until == 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			%s
			ORDER BY b.id DESC
			LIMIT $3
		`, query), jobName, pdb.ID, page.Limit)
		if err != nil {
			return nil, Pagination{}, err
		}
	} else if page.Until != 0 {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
			SELECT sub.*
			FROM (%s
					AND b.id > $3
				ORDER BY b.id ASC
				LIMIT $4
			) sub
			ORDER BY sub.id DESC
		`, query), jobName, pdb.ID, page.Until, page.Limit)
		if err != nil {
			return nil, Pagination{}, err
		}
	} else {
		rows, err = pdb.conn.Query(fmt.Sprintf(`
				%s
				AND b.id < $3
			ORDER BY b.id DESC
			LIMIT $4
		`, query), jobName, pdb.ID, page.Since, page.Limit)
		if err != nil {
			return nil, Pagination{}, err
		}
	}

	defer rows.Close()

	builds := []Build{}

	for rows.Next() {
		build, _, err := scanBuild(rows)
		if err != nil {
			return nil, Pagination{}, err
		}

		builds = append(builds, build)
	}

	if len(builds) == 0 {
		return []Build{}, Pagination{}, nil
	}

	err = pdb.conn.QueryRow(`
		SELECT COALESCE(MAX(b.id), 0) as maxID,
			COALESCE(MIN(b.id), 0) as minID
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		WHERE j.name = $1
			AND j.pipeline_id = $2
	`, jobName, pdb.ID).Scan(&maxID, &minID)
	if err != nil {
		return nil, Pagination{}, err
	}

	firstBuild = builds[0]
	lastBuild = builds[len(builds)-1]

	if firstBuild.ID < maxID {
		pagination.Previous = &Page{
			Until: firstBuild.ID,
			Limit: page.Limit,
		}
	}

	if lastBuild.ID > minID {
		pagination.Next = &Page{
			Since: lastBuild.ID,
			Limit: page.Limit,
		}
	}

	return builds, pagination, nil
}

func (pdb *pipelineDB) GetAllJobBuilds(job string) ([]Build, error) {
	rows, err := pdb.conn.Query(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
		INNER JOIN jobs j ON b.job_id = j.id
		INNER JOIN pipelines p ON j.pipeline_id = p.id
		WHERE j.name = $1
			AND j.pipeline_id = $2
		ORDER BY b.id DESC
	`, job, pdb.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	bs := []Build{}

	for rows.Next() {
		build, _, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		bs = append(bs, build)
	}

	return bs, nil
}

func (pdb *pipelineDB) GetJobFinishedAndNextBuild(job string) (*Build, *Build, error) {
	var finished *Build
	var next *Build

	finishedBuild, foundFinished, err := scanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
 		WHERE j.name = $1
			AND j.pipeline_id = $2
			AND b.status NOT IN ('pending', 'started')
		ORDER BY b.id DESC
		LIMIT 1
	`, job, pdb.ID))
	if err != nil {
		return nil, nil, err
	}

	if foundFinished {
		finished = &finishedBuild
	}

	nextBuild, foundNext, err := scanBuild(pdb.conn.QueryRow(`
		SELECT `+qualifiedBuildColumns+`
		FROM builds b
			INNER JOIN jobs j ON b.job_id = j.id
			INNER JOIN pipelines p ON j.pipeline_id = p.id
 		WHERE j.name = $1
			AND j.pipeline_id = $2
			AND status IN ('pending', 'started')
		ORDER BY b.id ASC
		LIMIT 1
	`, job, pdb.ID))
	if err != nil {
		return nil, nil, err
	}

	if foundNext {
		next = &nextBuild
	}

	return finished, next, nil
}

func (pdb *pipelineDB) GetDashboard() (Dashboard, atc.GroupConfigs, error) {
	pipelineConfig, _, _, err := pdb.GetConfig()
	if err != nil {
		return nil, nil, err
	}

	dashboard := Dashboard{}

	savedJobs, err := pdb.getJobs()
	if err != nil {
		return nil, nil, err
	}

	startedBuilds, err := pdb.getLastJobBuildsSatisfying("b.status = 'started'")
	if err != nil {
		return nil, nil, err
	}

	pendingBuilds, err := pdb.getLastJobBuildsSatisfying("b.status = 'pending'")
	if err != nil {
		return nil, nil, err
	}

	finishedBuilds, err := pdb.getLastJobBuildsSatisfying("b.status NOT IN ('pending', 'started')")
	if err != nil {
		return nil, nil, err
	}

	for _, job := range pipelineConfig.Jobs {
		savedJob, found := savedJobs[job.Name]
		if !found {
			return nil, nil, fmt.Errorf("found job in pipeline configuration but not in database: %s", job.Name)
		}

		dashboardJob := DashboardJob{
			Job:       savedJob,
			JobConfig: job,
		}

		if startedBuild, found := startedBuilds[job.Name]; found {
			dashboardJob.NextBuild = &startedBuild
		} else if pendingBuild, found := pendingBuilds[job.Name]; found {
			dashboardJob.NextBuild = &pendingBuild
		}

		if finishedBuild, found := finishedBuilds[job.Name]; found {
			dashboardJob.FinishedBuild = &finishedBuild
		}

		dashboard = append(dashboard, dashboardJob)
	}

	return dashboard, pipelineConfig.Groups, nil
}

func (pdb *pipelineDB) getJobs() (map[string]SavedJob, error) {
	rows, err := pdb.conn.Query(`
  	SELECT id, name, paused
  	FROM jobs
  	WHERE pipeline_id = $1
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	savedJobs := make(map[string]SavedJob)

	for rows.Next() {
		var savedJob SavedJob

		err := rows.Scan(&savedJob.ID, &savedJob.Name, &savedJob.Paused)
		if err != nil {
			return nil, err
		}

		savedJob.PipelineName = pdb.Name

		savedJobs[savedJob.Name] = savedJob
	}

	return savedJobs, nil
}

func (pdb *pipelineDB) getLastJobBuildsSatisfying(bRequirement string) (map[string]Build, error) {
	rows, err := pdb.conn.Query(`
		 SELECT `+qualifiedBuildColumns+`
		 FROM builds b, jobs j, pipelines p,
			 (
				 SELECT b.job_id AS job_id, MAX(b.id) AS id
				 FROM builds b, jobs j
				 WHERE b.job_id = j.id
					 AND `+bRequirement+`
					 AND j.pipeline_id = $1
				 GROUP BY b.job_id
			 ) max
		 WHERE b.job_id = j.id
			 AND b.id = max.id
			 AND p.id = $1
			 AND j.pipeline_id = p.id
  `, pdb.ID)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	nextBuilds := make(map[string]Build)

	for rows.Next() {
		var build Build

		build, scanned, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}

		if !scanned {
			return nil, errors.New("row could not be scanned")
		}

		nextBuilds[build.JobName] = build
	}

	return nextBuilds, nil
}

func (pdb *pipelineDB) getJob(tx Tx, name string) (SavedJob, error) {
	var job SavedJob

	err := tx.QueryRow(`
  	SELECT id, name, paused
  	FROM jobs
  	WHERE name = $1
  		AND pipeline_id = $2
  `, name, pdb.ID).Scan(&job.ID, &job.Name, &job.Paused)
	if err != nil {
		return SavedJob{}, err
	}

	job.PipelineName = pdb.Name

	return job, nil
}

func (pdb *pipelineDB) getJobByID(id int) (SavedJob, error) {
	var job SavedJob

	err := pdb.conn.QueryRow(`
		SELECT id, name, paused
		FROM jobs
		WHERE id = $1
  `, id).Scan(&job.ID, &job.Name, &job.Paused)
	if err != nil {
		return SavedJob{}, err
	}

	job.PipelineName = pdb.Name

	return job, nil
}
