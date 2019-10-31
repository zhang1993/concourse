package migration_test

import (
	"database/sql"
	"io/ioutil"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/concourse/atc/db/encryption"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/migration/migrationfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("SchemaAdapter", func() {
	var (
		err           error
		db            *sql.DB
		lockDB        *sql.DB
		lockFactory   lock.LockFactory
		strategy      encryption.Strategy
		bindata       *migrationfakes.FakeBindata
		schemaAdapter *migration.OldSchemaMigrationAdapter
		fakeLogFunc   = func(logger lager.Logger, id lock.LockID) {}
		logger        lager.Logger
	)

	JustBeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		lockDB, err = sql.Open("postgres", postgresRunner.DataSourceName())
		Expect(err).NotTo(HaveOccurred())

		lockFactory = lock.NewLockFactory(lockDB, fakeLogFunc, fakeLogFunc)
		strategy = encryption.NewNoEncryption()

		bindata = new(migrationfakes.FakeBindata)
		bindata.AssetStub = asset

		schemaAdapter = OldSchemaMigrationAdapter{}
	})

	AfterEach(func() {
		_ = db.Close()
		_ = lockDB.Close()
	})

	Context("schema_migrations table exists", func() {
		var (
			version int
			dirty   bool
			err     error
		)
		JustBeforeEach(func() {
			SetupSchemaMigrationsTable(db, version, dirty)
			_, err = schemaAdapter.MigrateFromOldSchema(db, 1234)
		})
		Context("migrating up", func() {
			Context("dirty state", func() {
				BeforeEach(func() {
					dirty = true
					version = 123
				})
				It("fails with an error stating that db is in a dirty state", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("database is dirty"))
				})
			})
			It("migrates up to the highest version supported for schema_migrations", func() {
				Fail("not implemented")
			})
		})

		Context("migrating down", func() {
			It("migrates down to the lowest version supported for schema_migrations", func() {
				Fail("not implemented")
			})
		})

		Context("current version", func() {
			It("reports the current version as given in schema_migrations", func() {
				Fail("not implemented")
			})
		})
	})

	Context("schema_migrations table does not exist", func() {
		Context("migrating up", func() {
			It("does nothing", func() {
				Fail("not implemented")
			})
		})

		Context("migrating down", func() {
			It("does nothing", func() {
				Fail("not implemented")
			})
		})

		Context("current version", func() {
			It("does nothing", func() {
				Fail("not implemented")
			})
		})
	})
})

func ExpectMigrationVersionTableNotToExist(dbConn *sql.DB) {
	var exists string
	err := dbConn.QueryRow("SELECT EXISTS(SELECT 1 FROM information_schema.tables where table_name = 'migration_version')").Scan(&exists)
	Expect(err).NotTo(HaveOccurred())
	Expect(exists).To(Equal("false"))
}

func ExpectDatabaseVersionToEqual(db *sql.DB, version int, table string) {
	var dbVersion int
	query := "SELECT version from " + table + " LIMIT 1"
	err := db.QueryRow(query).Scan(&dbVersion)
	Expect(err).NotTo(HaveOccurred())
	Expect(dbVersion).To(Equal(version))
}

func SetupSchemaMigrationsTable(db *sql.DB, version int, dirty bool) {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS schema_migrations (version bigint, dirty boolean)")
	Expect(err).NotTo(HaveOccurred())
	_, err = db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES ($1, $2)", version, dirty)
	Expect(err).NotTo(HaveOccurred())
}

func SetupSchemaFromFile(db *sql.DB, path string) {
	migrations, err := ioutil.ReadFile(path)
	Expect(err).NotTo(HaveOccurred())

	for _, migration := range strings.Split(string(migrations), ";") {
		_, err = db.Exec(migration)
		Expect(err).NotTo(HaveOccurred())
	}
}

func ExpectToBeAbleToInsertData(dbConn *sql.DB) {
	rand.Seed(time.Now().UnixNano())

	teamID := rand.Intn(10000)
	_, err := dbConn.Exec("INSERT INTO teams(id, name) VALUES ($1, $2)", teamID, strconv.Itoa(teamID))
	Expect(err).NotTo(HaveOccurred())

	pipelineID := rand.Intn(10000)
	_, err = dbConn.Exec("INSERT INTO pipelines(id, team_id, name) VALUES ($1, $2, $3)", pipelineID, teamID, strconv.Itoa(pipelineID))
	Expect(err).NotTo(HaveOccurred())

	jobID := rand.Intn(10000)
	_, err = dbConn.Exec("INSERT INTO jobs(id, pipeline_id, name, config) VALUES ($1, $2, $3, '{}')", jobID, pipelineID, strconv.Itoa(jobID))
	Expect(err).NotTo(HaveOccurred())
}
