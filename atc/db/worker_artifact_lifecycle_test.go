package db_test

import (
	"fmt"
	"time"

	"code.cloudfoundry.org/lager/lagertest"
	"github.com/cloudfoundry/bosh-cli/director/template"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/creds"
	"github.com/concourse/concourse/atc/db"
)

var _ = Describe("WorkerArtifactLifecycle", func() {
	var workerArtifactLifecycle db.WorkerArtifactLifecycle
	var testLogger lagertest.TestLogger

	BeforeEach(func() {
		workerArtifactLifecycle = db.NewArtifactLifecycle(dbConn)
	})

	Describe("CreateArtifact", func() {
		It("adds a new artifact record to the db", func() {
			artifact, err := workerArtifactLifecycle.CreateArtifact("some-artifact-name")
			Expect(err).ToNot(HaveOccurred())
			Expect(artifact.Name()).To(Equal("some-artifact-name"))

			result, err := dbConn.Exec("SELECT * from worker_artifacts")
			Expect(err).ToNot(HaveOccurred())
			Expect(result.RowsAffected()).To(BeEquivalentTo(1))
		})

	})

	Describe("RemoveExpiredArtifacts", func() {
		var initialized bool

		JustBeforeEach(func() {
			_, err := dbConn.Exec("INSERT INTO worker_artifacts(name, created_at, initialized) VALUES('old-artifact', NOW() - '13 hours'::interval, $1)", initialized)
			Expect(err).ToNot(HaveOccurred())

			_, err = dbConn.Exec("INSERT INTO worker_artifacts(name, created_at, initialized) VALUES('young-artifact', NOW(), $1)", initialized)
			Expect(err).ToNot(HaveOccurred())

			err = workerArtifactLifecycle.RemoveExpiredArtifacts(testLogger)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("uninitialized artifacts", func() {
			BeforeEach(func() {
				initialized = false
			})

			It("removes artifacts created more than 12 hours ago", func() {
				var artifactNames []string
				rows, err := dbConn.Query("SELECT name from worker_artifacts")
				Expect(err).ToNot(HaveOccurred())

				for rows.Next() {
					var name string
					err = rows.Scan(&name)
					Expect(err).ToNot(HaveOccurred())
					artifactNames = append(artifactNames, name)
				}

				Expect(len(artifactNames)).To(Equal(1))
				Expect(artifactNames).Should(ConsistOf("young-artifact"))
			})
		})

		Context("artifacts are initialized", func() {
			BeforeEach(func() {
				initialized = true
			})

			It("does not delete any record", func() {
				var count int
				err := dbConn.QueryRow("SELECT count(*) from worker_artifacts").Scan(&count)
				Expect(err).ToNot(HaveOccurred())
				Expect(count).To(Equal(2))
			})

		})
	})

	Describe("RemoveUnassociatedWorkerArtifacts", func() {
		JustBeforeEach(func() {
			err := workerArtifactLifecycle.RemoveOrphanedArtifacts(testLogger)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("artifacts are initialized", func() {
			var initialized = true
			var expectedArtifactNames = []string{"artifact-with-association"}

			Context("worker resource certs", func() {
				BeforeEach(func() {
					tx, err := dbConn.Begin()
					Expect(err).NotTo(HaveOccurred())

					workerResourceCerts, err := db.WorkerResourceCerts{
						WorkerName: defaultWorker.Name(),
						CertsPath:  "/etc/blah/blah/certs",
					}.FindOrCreate(tx)
					Expect(err).ToNot(HaveOccurred())

					_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, worker_resource_certs_id, initialized) VALUES('artifact-with-association', NOW() - '1 hour'::interval, $1, $2)",
						workerResourceCerts.ID, initialized)
					Expect(err).ToNot(HaveOccurred())

					_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, initialized) VALUES('unassociated-artifact', NOW() - '1 hour'::interval, $1)", initialized)
					Expect(err).ToNot(HaveOccurred())

					err = tx.Commit()
					Expect(err).ToNot(HaveOccurred())

				})

				It("only removes initialized artifacts that have no associations", func() {
					rows, err := dbConn.Query("SELECT name from worker_artifacts")
					Expect(err).ToNot(HaveOccurred())

					var artifactNames []string
					var artifactName string

					for rows.Next() {
						err = rows.Scan(&artifactName)
						Expect(err).ToNot(HaveOccurred())
						artifactNames = append(artifactNames, artifactName)
					}
					Expect(artifactNames).Should(ConsistOf(expectedArtifactNames))
				})
			})

			Context("base resource types", func() {
				BeforeEach(func() {
					baseResourceType, found, err := workerBaseResourceTypeFactory.Find(
						"some-base-resource-type",
						defaultWorker,
					)
					Expect(err).ToNot(HaveOccurred())
					Expect(found).To(BeTrue())
					_, err = dbConn.Exec("INSERT INTO worker_artifacts(name, created_at, worker_base_resource_type_id, initialized) VALUES('artifact-with-association', NOW() - '1 hour'::interval, $1, $2)", baseResourceType.ID, initialized)
					Expect(err).ToNot(HaveOccurred())

					_, err = dbConn.Exec("INSERT INTO worker_artifacts(name, created_at, initialized) VALUES('unassociated-artifact', NOW() - '1 hour'::interval, $1)", initialized)
					Expect(err).ToNot(HaveOccurred())

				})

				It("only removes initialized artifacts that have no associations", func() {
					rows, err := dbConn.Query("SELECT name from worker_artifacts")
					Expect(err).ToNot(HaveOccurred())

					var artifactNames []string
					var artifactName string

					for rows.Next() {
						err = rows.Scan(&artifactName)
						Expect(err).ToNot(HaveOccurred())
						artifactNames = append(artifactNames, artifactName)
					}
					Expect(artifactNames).Should(ConsistOf(expectedArtifactNames))
				})
			})

			Context("task caches", func() {
				BeforeEach(func() {
					usedTaskCache, err := workerTaskCacheFactory.FindOrCreate(
						defaultJob.ID(),
						"somestep",
						"/some/task/cache",
						defaultWorker.Name(),
					)
					Expect(err).ToNot(HaveOccurred())

					_, err = dbConn.Exec("INSERT INTO worker_artifacts(name, created_at, worker_task_cache_id, initialized) VALUES('artifact-with-association', NOW() - '1 hour'::interval, $1, $2)", usedTaskCache.ID, initialized)
					Expect(err).ToNot(HaveOccurred())

					_, err = dbConn.Exec("INSERT INTO worker_artifacts(name, created_at, initialized) VALUES('unassociated-artifact', NOW() - '1 hour'::interval, $1)", initialized)
					Expect(err).ToNot(HaveOccurred())

				})

				It("only removes initialized artifacts that have no associations", func() {
					rows, err := dbConn.Query("SELECT name from worker_artifacts")
					Expect(err).ToNot(HaveOccurred())

					var artifactNames []string
					var artifactName string

					for rows.Next() {
						err = rows.Scan(&artifactName)
						Expect(err).ToNot(HaveOccurred())
						artifactNames = append(artifactNames, artifactName)
					}
					Expect(artifactNames).Should(ConsistOf(expectedArtifactNames))
				})
			})

			Context("resource caches", func() {
				BeforeEach(func() {
					build, err := defaultTeam.CreateOneOffBuild()
					Expect(err).ToNot(HaveOccurred())

					resourceCache, err := resourceCacheFactory.FindOrCreateResourceCache(
						logger,
						db.ForBuild(build.ID()),
						"some-base-resource-type",
						atc.Version{"some": "version"},
						atc.Source{"some": "source"},
						atc.Params{},
						creds.VersionedResourceTypes{},
					)
					Expect(err).ToNot(HaveOccurred())

					workerResourceCache := db.WorkerResourceCache{
						ResourceCache: resourceCache,
						WorkerName:    defaultWorker.Name(),
					}

					tx, err := dbConn.Begin()
					Expect(err).ToNot(HaveOccurred())
					defer tx.Rollback()

					usedCache, err := workerResourceCache.FindOrCreate(tx)
					Expect(err).ToNot(HaveOccurred())

					_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, worker_resource_cache_id, initialized) VALUES('artifact-with-association', NOW() - '1 hour'::interval, $1, $2)", usedCache.ID, initialized)
					Expect(err).ToNot(HaveOccurred())

					_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, initialized) VALUES('unassociated-artifact', NOW() - '1 hour'::interval, $1)", initialized)
					Expect(err).ToNot(HaveOccurred())

					err = tx.Commit()
					Expect(err).ToNot(HaveOccurred())
				})

				It("only removes initialized artifacts that have no associations", func() {
					rows, err := dbConn.Query("SELECT name from worker_artifacts")
					Expect(err).ToNot(HaveOccurred())

					var artifactNames []string
					var artifactName string

					for rows.Next() {
						err = rows.Scan(&artifactName)
						Expect(err).ToNot(HaveOccurred())
						artifactNames = append(artifactNames, artifactName)
					}
					Expect(artifactNames).Should(ConsistOf(expectedArtifactNames))
				})
			})

			Context("when the associated build has terminated", func() {
				Context("errored build status", func() {
					BeforeEach(func() {
						erroredBuild, err := defaultTeam.CreateOneOffBuild()
						Expect(err).ToNot(HaveOccurred())
						err = erroredBuild.FinishWithError(fmt.Errorf("uh oh"))
						Expect(err).ToNot(HaveOccurred())

						succeededBuild, err := defaultTeam.CreateOneOffBuild()
						Expect(err).ToNot(HaveOccurred())
						err = succeededBuild.Finish(db.BuildStatusSucceeded)
						Expect(err).ToNot(HaveOccurred())

						abortedBuild, err := defaultTeam.CreateOneOffBuild()
						Expect(err).ToNot(HaveOccurred())
						err = abortedBuild.MarkAsAborted()
						Expect(err).ToNot(HaveOccurred())

						failedBuild, err := defaultTeam.CreateOneOffBuild()
						Expect(err).ToNot(HaveOccurred())
						err = failedBuild.Finish(db.BuildStatusFailed)
						Expect(err).ToNot(HaveOccurred())

						pendingBuild, err := defaultTeam.CreateOneOffBuild()
						Expect(err).ToNot(HaveOccurred())

						startedBuild, err := defaultTeam.CreateOneOffBuild()
						Expect(err).ToNot(HaveOccurred())

						tx, err := dbConn.Begin()
						Expect(err).ToNot(HaveOccurred())
						defer tx.Rollback()

						_, err = tx.Exec("UPDATE builds SET status='started' where id=$1", startedBuild.ID())
						Expect(err).ToNot(HaveOccurred())

						_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, build_id, initialized) VALUES('artifact-with-errored-build', NOW() - '1 hour'::interval, $1, $2)", erroredBuild.ID(), true)
						Expect(err).ToNot(HaveOccurred())

						_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, build_id, initialized) VALUES('artifact-with-succeeded-build', NOW() - '1 hour'::interval, $1, $2)", succeededBuild.ID(), true)
						Expect(err).ToNot(HaveOccurred())

						_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, build_id, initialized) VALUES('artifact-with-started-build', NOW() - '1 hour'::interval, $1, $2)", startedBuild.ID(), true)
						Expect(err).ToNot(HaveOccurred())

						_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, build_id, initialized) VALUES('artifact-with-aborted-build', NOW() - '1 hour'::interval, $1, $2)", abortedBuild.ID(), true)
						Expect(err).ToNot(HaveOccurred())

						_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, build_id, initialized) VALUES('artifact-with-failed-build', NOW() - '1 hour'::interval, $1, $2)", failedBuild.ID(), true)
						Expect(err).ToNot(HaveOccurred())

						_, err = tx.Exec("INSERT INTO worker_artifacts(name, created_at, build_id, initialized) VALUES('artifact-with-pending-build', NOW() - '1 hour'::interval, $1, $2)", pendingBuild.ID(), true)
						Expect(err).ToNot(HaveOccurred())

						err = tx.Commit()
						Expect(err).ToNot(HaveOccurred())

					})

					// NOTE: This test does not yet cover builds that might be hijacked
					It("does not remove artifacts owned by in-progress builds ", func() {
						rows, err := dbConn.Query("SELECT name from worker_artifacts")
						Expect(err).ToNot(HaveOccurred())

						var artifactNames []string
						var artifactName string

						for rows.Next() {
							err = rows.Scan(&artifactName)
							Expect(err).ToNot(HaveOccurred())
							artifactNames = append(artifactNames, artifactName)
						}
						Expect(artifactNames).Should(ConsistOf([]string{"artifact-with-started-build", "artifact-with-pending-build"}))

					})
				})
			})
		})

		Context("artifact is not initialized", func() {
			BeforeEach(func() {
				_, err := dbConn.Exec("INSERT INTO worker_artifacts(name, created_at, initialized) VALUES('artifact', NOW() - '1 hour'::interval, $1)", false)
				Expect(err).ToNot(HaveOccurred())
			})

			It("does not remove any artifacts", func() {
				rows, err := dbConn.Query("SELECT name from worker_artifacts")
				Expect(err).ToNot(HaveOccurred())

				var artifactNames []string
				var artifactName string

				for rows.Next() {
					err = rows.Scan(&artifactName)
					Expect(err).ToNot(HaveOccurred())
					artifactNames = append(artifactNames, artifactName)
				}
				Expect(artifactNames).Should(ConsistOf("artifact"))
			})
		})
	})

	Describe("FindArtifactForResourceCache", func() {
		var (
			cache                   db.UsedResourceCache
			usedWorkerResourceCache *db.UsedWorkerResourceCache
			artifact                db.WorkerArtifact
			foundArtifact           db.WorkerArtifact
			err                     error
		)

		BeforeEach(func() {
			build, err := defaultJob.CreateBuild()
			Expect(err).ToNot(HaveOccurred())
			cache, err = resourceCacheFactory.FindOrCreateResourceCache(
				testLogger,
				db.ForBuild(build.ID()),
				"some-base-resource-type",
				atc.Version{"some": "version"},
				atc.Source{
					"some": "source",
				},
				atc.Params{"some": fmt.Sprintf("param-%d", time.Now().UnixNano())},
				creds.NewVersionedResourceTypes(
					template.StaticVariables{"source-param": "some-secret-sauce"},
					atc.VersionedResourceTypes{},
				),
			)
			Expect(err).ToNot(HaveOccurred())
			workerResourceCache := &db.WorkerResourceCache{
				defaultWorker.Name(),
				cache,
			}
			tx, err := dbConn.Begin()
			usedWorkerResourceCache, err = workerResourceCache.FindOrCreate(tx)
			Expect(err).ToNot(HaveOccurred())
			err = tx.Commit()
			Expect(err).ToNot(HaveOccurred())

			artifact, err = workerArtifactLifecycle.CreateArtifact("some-artifact")
			Expect(err).ToNot(HaveOccurred())
			Expect(artifact).ToNot(BeNil())
			err = artifact.AttachToResourceCache(usedWorkerResourceCache.ID)
			Expect(err).ToNot(HaveOccurred())
		})

		It("returns the artifact associated with the resource cache", func() {
			foundArtifact, err = workerArtifactLifecycle.FindArtifactForResourceCache(testLogger, cache.ID())
			Expect(err).ToNot(HaveOccurred())
			Expect(foundArtifact).ToNot(BeNil())
			Expect(foundArtifact.ID()).To(Equal(artifact.ID()))
		})
	})
})
