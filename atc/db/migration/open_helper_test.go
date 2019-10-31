package migration_test

// import (
// 	"database/sql"

// 	"code.cloudfoundry.org/lager"
// 	"code.cloudfoundry.org/lager/lagertest"

// 	"github.com/concourse/concourse/atc/db/encryption"
// 	"github.com/concourse/concourse/atc/db/lock"
// 	"github.com/concourse/concourse/atc/db/migration"
// 	"github.com/concourse/concourse/atc/db/migration/migrationfakes"
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// )

// var _ = XDescribe("OpenHelper", func() {
// 	var (
// 		err         error
// 		db          *sql.DB
// 		lockDB      *sql.DB
// 		lockFactory lock.LockFactory
// 		strategy    encryption.Strategy
// 		bindata     *migrationfakes.FakeBindata
// 		openHelper  *migration.OpenHelper
// 		fakeLogFunc = func(logger lager.Logger, id lock.LockID) {}
// 		logger      lager.Logger
// 	)

// 	JustBeforeEach(func() {
// 		logger = lagertest.NewTestLogger("test")
// 		db, err = sql.Open("postgres", postgresRunner.DataSourceName())
// 		Expect(err).NotTo(HaveOccurred())

// 		lockDB, err = sql.Open("postgres", postgresRunner.DataSourceName())
// 		Expect(err).NotTo(HaveOccurred())

// 		lockFactory = lock.NewLockFactory(lockDB, fakeLogFunc, fakeLogFunc)
// 		strategy = encryption.NewNoEncryption()
// 		openHelper = migration.NewOpenHelper("postgres", postgresRunner.DataSourceName(), lockFactory, strategy)

// 		bindata = new(migrationfakes.FakeBindata)
// 		bindata.AssetStub = asset
// 	})

// 	AfterEach(func() {
// 		_ = db.Close()
// 		_ = lockDB.Close()
// 	})

// Context("legacy migration_version table exists", func() {
// 	It("Fails if trying to upgrade from a migration_version < 189", func() {
// 		SetupMigrationVersionTableToExistAtVersion(db, 188)

// 		err = openHelper.MigrateToVersion(logger, 1554469235)

// 		Expect(err.Error()).To(Equal("Must upgrade from db version 189 (concourse 3.6.0), current db version: 188"))

// 		_, err = db.Exec("SELECT version FROM migration_version")
// 		Expect(err).NotTo(HaveOccurred())
// 	})

// 	It("Fails if trying to upgrade from a migration_version > 189", func() {
// 		SetupMigrationVersionTableToExistAtVersion(db, 190)

// 		err = openHelper.MigrateToVersion(5000)

// 		Expect(err.Error()).To(Equal("Must upgrade from db version 189 (concourse 3.6.0), current db version: 190"))

// 		_, err = db.Exec("SELECT version FROM migration_version")
// 		Expect(err).NotTo(HaveOccurred())
// 	})

// 	It("Forces schema migration version to a known first version if migration_version is 189", func() {
// 		var initialSchemaVersion = 1510262030
// 		SetupMigrationVersionTableToExistAtVersion(db, 189)

// 		SetupSchemaFromFile(db, "migrations/1510262030_initial_schema.up.sql")

// 		err = openHelper.MigrateToVersion(logger, initialSchemaVersion)
// 		Expect(err).NotTo(HaveOccurred())

// 		ExpectDatabaseVersionToEqual(db, initialSchemaVersion, "schema_migrations")

// 		ExpectMigrationVersionTableNotToExist(db)

// 		ExpectToBeAbleToInsertData(db)
// 	})

// 	It("Runs migrator if migration_version table does not exist", func() {

// 		bindata.AssetNamesReturns([]string{
// 			"1510262030_initial_schema.up.sql",
// 		})
// 		err = openHelper.MigrateToVersion(logger, initialSchemaVersion)
// 		Expect(err).NotTo(HaveOccurred())

// 		ExpectDatabaseVersionToEqual(db, initialSchemaVersion, "migrations_history")

// 		ExpectMigrationVersionTableNotToExist(db)

// 		ExpectToBeAbleToInsertData(db)
// 	})

// })
// })
