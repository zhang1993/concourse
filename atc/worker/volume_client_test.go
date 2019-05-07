package worker_test

import (
	"errors"
	"time"

	"code.cloudfoundry.org/clock/fakeclock"
	"code.cloudfoundry.org/lager"
	"code.cloudfoundry.org/lager/lagertest"
	"github.com/concourse/baggageclaim"
	"github.com/concourse/baggageclaim/baggageclaimfakes"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/db/lock"
	"github.com/concourse/concourse/atc/db/lock/lockfakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = XDescribe("VolumeClient", func() {
	var (
		fakeLock   *lockfakes.FakeLock
		testLogger *lagertest.TestLogger

		fakeBaggageclaimClient            *baggageclaimfakes.FakeClient
		fakeLockFactory                   *lockfakes.FakeLockFactory
		fakeDBVolumeRepository            *dbfakes.FakeVolumeRepository
		fakeWorkerBaseResourceTypeFactory *dbfakes.FakeWorkerBaseResourceTypeFactory
		fakeWorkerTaskCacheFactory        *dbfakes.FakeWorkerTaskCacheFactory
		fakeClock                         *fakeclock.FakeClock
		dbWorker                          *dbfakes.FakeWorker

		volumeClient worker.VolumeClient
	)

	BeforeEach(func() {
		fakeBaggageclaimClient = new(baggageclaimfakes.FakeClient)
		fakeLockFactory = new(lockfakes.FakeLockFactory)
		fakeClock = fakeclock.NewFakeClock(time.Unix(123, 456))
		dbWorker = new(dbfakes.FakeWorker)
		dbWorker.NameReturns("some-worker")

		testLogger = lagertest.NewTestLogger("test")

		fakeDBVolumeRepository = new(dbfakes.FakeVolumeRepository)
		fakeWorkerBaseResourceTypeFactory = new(dbfakes.FakeWorkerBaseResourceTypeFactory)
		fakeWorkerTaskCacheFactory = new(dbfakes.FakeWorkerTaskCacheFactory)
		fakeLock = new(lockfakes.FakeLock)

		volumeClient = worker.NewVolumeClient(
			fakeBaggageclaimClient,
			dbWorker,
			fakeClock,

			fakeLockFactory,
			fakeDBVolumeRepository,
			fakeWorkerBaseResourceTypeFactory,
			fakeWorkerTaskCacheFactory,
		)
	})

	Describe("FindOrCreateVolumeForContainer", func() {
		var fakeBaggageclaimVolume *baggageclaimfakes.FakeVolume
		var foundOrCreatedVolume worker.Artifact
		var foundOrCreatedErr error
		var container db.CreatingContainer
		var fakeCreatingVolume *dbfakes.FakeCreatingVolume
		var volumeStrategy baggageclaim.Strategy

		BeforeEach(func() {
			fakeBaggageclaimVolume = new(baggageclaimfakes.FakeVolume)
			fakeCreatingVolume = new(dbfakes.FakeCreatingVolume)
			fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)
			fakeDBVolumeRepository.CreateContainerVolumeReturns(fakeCreatingVolume, nil)

			volumeStrategy = baggageclaim.ImportStrategy{
				Path: "/some/path",
			}
		})

		JustBeforeEach(func() {
			container = new(dbfakes.FakeCreatingContainer)
			foundOrCreatedVolume, foundOrCreatedErr = volumeClient.FindOrCreateVolumeForContainer(
				testLogger,
				worker.VolumeSpec{
					Strategy: volumeStrategy,
				},
				container,
				42,
				"some-mount-path",
			)
		})

		Context("when artifact exists in creating state", func() {
			BeforeEach(func() {
				fakeDBVolumeRepository.FindContainerVolumeReturns(fakeCreatingVolume, nil, nil)
			})

			Context("when acquiring artifact creating lock fails", func() {
				var disaster = errors.New("disaster")

				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, false, disaster)
				})

				It("returns error", func() {
					Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
					Expect(foundOrCreatedErr).To(Equal(disaster))
				})
			})

			Context("when it could not acquire creating lock", func() {
				BeforeEach(func() {
					callCount := 0
					fakeLockFactory.AcquireStub = func(logger lager.Logger, lockID lock.LockID) (lock.Lock, bool, error) {
						callCount++
						go fakeClock.WaitForWatcherAndIncrement(time.Second)

						if callCount < 3 {
							return nil, false, nil
						}

						return fakeLock, true, nil
					}
				})

				It("retries to find artifact again", func() {
					Expect(fakeLockFactory.AcquireCallCount()).To(Equal(3))
					Expect(fakeDBVolumeRepository.FindContainerVolumeCallCount()).To(Equal(3))
				})
			})

			Context("when it acquires the lock", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(fakeLock, true, nil)
				})

				Context("when checking for the artifact in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeStub = func(lager.Logger, string) (baggageclaim.Volume, bool, error) {
							Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
							return nil, false, nil
						}
					})

					It("does so with the lock held", func() {
						Expect(fakeBaggageclaimClient.LookupVolumeCallCount()).To(Equal(1))
					})
				})

				Context("when artifact exists in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
					})

					It("returns the artifact", func() {
						Expect(foundOrCreatedErr).NotTo(HaveOccurred())
						Expect(foundOrCreatedVolume).NotTo(BeNil())
					})
				})

				Context("when artifact does not exist in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
					})

					It("creates artifact in baggageclaim", func() {
						Expect(foundOrCreatedErr).NotTo(HaveOccurred())
						Expect(foundOrCreatedVolume).NotTo(BeNil())
						Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
					})

					Context("when creating the artifact in baggageclaim fails", func() {
						BeforeEach(func() {
							fakeBaggageclaimClient.CreateVolumeReturns(nil, errors.New("failed to create artifact, oh no"))
						})

						It("marks the creating artifact as failed", func() {
							Expect(fakeCreatingVolume.FailedCallCount()).To(Equal(1))
						})
					})
				})

				It("releases the lock", func() {
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
			})
		})

		Context("when artifact exists in created state", func() {
			BeforeEach(func() {
				fakeCreatedVolume := new(dbfakes.FakeCreatedVolume)
				fakeCreatedVolume.HandleReturns("fake-handle")
				fakeCreatedVolume.WorkerNameReturns("fake-worker")
				fakeDBVolumeRepository.FindContainerVolumeReturns(nil, fakeCreatedVolume, nil)
			})

			Context("when artifact exists in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
				})

				It("returns the artifact", func() {
					Expect(foundOrCreatedErr).NotTo(HaveOccurred())
					Expect(foundOrCreatedVolume).NotTo(BeNil())
				})
			})

			Context("when artifact does not exist in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(foundOrCreatedErr).To(HaveOccurred())
					Expect(foundOrCreatedErr.Error()).To(ContainSubstring("artifact 'fake-handle' disappeared from worker 'fake-worker'"))
				})
			})
		})

		Context("when artifact does not exist in db", func() {
			var fakeCreatedVolume *dbfakes.FakeCreatedVolume

			BeforeEach(func() {
				fakeDBVolumeRepository.FindContainerVolumeReturns(nil, nil, nil)
				fakeLockFactory.AcquireReturns(fakeLock, true, nil)
				fakeDBVolumeRepository.CreateContainerVolumeReturns(fakeCreatingVolume, nil)
				fakeCreatedVolume = new(dbfakes.FakeCreatedVolume)
				fakeCreatingVolume.CreatedReturns(fakeCreatedVolume, nil)
			})

			It("acquires the lock", func() {
				Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
			})

			It("creates artifact in creating state", func() {
				Expect(fakeDBVolumeRepository.CreateContainerVolumeCallCount()).To(Equal(1))
				actualTeamID, actualWorkerName, actualContainer, actualMountPath := fakeDBVolumeRepository.CreateContainerVolumeArgsForCall(0)
				Expect(actualTeamID).To(Equal(42))
				Expect(actualWorkerName).To(Equal(dbWorker.Name()))
				Expect(actualContainer).To(Equal(container))
				Expect(actualMountPath).To(Equal("some-mount-path"))
			})

			It("creates artifact in baggageclaim", func() {
				Expect(foundOrCreatedErr).NotTo(HaveOccurred())
				Expect(foundOrCreatedVolume).To(Equal(worker.NewArtifact(fakeBaggageclaimVolume, fakeCreatedVolume, volumeClient)))
				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
			})

			Context("when creating the artifact in baggageclaim fails", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.CreateVolumeReturns(nil, errors.New("failed to create artifact, oh no"))
				})

				It("marks the creating artifact for removal", func() {
					Expect(fakeCreatingVolume.FailedCallCount()).To(Equal(1))
				})
			})
		})
	})

	Describe("FindOrCreateCOWVolumeForContainer", func() {
		var fakeBaggageclaimVolume *baggageclaimfakes.FakeVolume
		var foundOrCreatedVolume worker.Artifact
		var foundOrCreatedErr error
		var container db.CreatingContainer
		var fakeCreatingVolume *dbfakes.FakeCreatingVolume
		var fakeCreatedVolume *dbfakes.FakeCreatedVolume
		var volumeStrategy baggageclaim.Strategy
		var parentVolume *workerfakes.FakeVolume
		var fakeParentBCVolume *baggageclaimfakes.FakeVolume

		BeforeEach(func() {
			parentVolume = new(workerfakes.FakeVolume)
			parentVolume.HandleReturns("fake-parent-handle")

			fakeParentBCVolume = new(baggageclaimfakes.FakeVolume)

			volumeStrategy = baggageclaim.COWStrategy{
				Parent: fakeParentBCVolume,
			}

			fakeCreatingVolume = new(dbfakes.FakeCreatingVolume)
			parentVolume.CreateChildForContainerReturns(fakeCreatingVolume, nil)

			fakeBaggageclaimVolume = new(baggageclaimfakes.FakeVolume)
			fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)

			fakeCreatedVolume = new(dbfakes.FakeCreatedVolume)
			fakeCreatingVolume.CreatedReturns(fakeCreatedVolume, nil)
		})

		JustBeforeEach(func() {
			container = new(dbfakes.FakeCreatingContainer)
			foundOrCreatedVolume, foundOrCreatedErr = volumeClient.FindOrCreateCOWVolumeForContainer(
				testLogger,
				worker.VolumeSpec{
					Strategy: volumeStrategy,
				},
				container,
				parentVolume,
				42,
				"some-mount-path",
			)
		})

		Context("when artifact exists in creating state", func() {
			BeforeEach(func() {
				fakeDBVolumeRepository.FindContainerVolumeReturns(fakeCreatingVolume, nil, nil)
			})

			Context("when acquiring artifact creating lock fails", func() {
				var disaster = errors.New("disaster")

				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(nil, false, disaster)
				})

				It("returns error", func() {
					Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
					Expect(foundOrCreatedErr).To(Equal(disaster))
				})
			})

			Context("when it could not acquire creating lock", func() {
				BeforeEach(func() {
					callCount := 0
					fakeLockFactory.AcquireStub = func(logger lager.Logger, lockID lock.LockID) (lock.Lock, bool, error) {
						callCount++
						go fakeClock.WaitForWatcherAndIncrement(time.Second)

						if callCount < 3 {
							return nil, false, nil
						}

						return fakeLock, true, nil
					}
				})

				It("retries to find artifact again", func() {
					Expect(fakeLockFactory.AcquireCallCount()).To(Equal(3))
					Expect(fakeDBVolumeRepository.FindContainerVolumeCallCount()).To(Equal(3))
				})
			})

			Context("when it acquires the lock", func() {
				BeforeEach(func() {
					fakeLockFactory.AcquireReturns(fakeLock, true, nil)
				})

				Context("when artifact exists in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
					})

					It("returns the artifact", func() {
						Expect(foundOrCreatedErr).NotTo(HaveOccurred())
						Expect(foundOrCreatedVolume).NotTo(BeNil())
					})
				})

				Context("when artifact does not exist in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
					})

					It("creates artifact in baggageclaim", func() {
						Expect(foundOrCreatedErr).NotTo(HaveOccurred())
						Expect(foundOrCreatedVolume).NotTo(BeNil())
						Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
					})
				})

				It("releases the lock", func() {
					Expect(fakeLock.ReleaseCallCount()).To(Equal(1))
				})
			})
		})

		Context("when artifact exists in created state", func() {
			BeforeEach(func() {
				fakeCreatedVolume := new(dbfakes.FakeCreatedVolume)
				fakeCreatedVolume.HandleReturns("fake-handle")
				fakeCreatedVolume.WorkerNameReturns("fake-worker")
				fakeDBVolumeRepository.FindContainerVolumeReturns(nil, fakeCreatedVolume, nil)
			})

			Context("when artifact exists in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
				})

				It("returns the artifact", func() {
					Expect(foundOrCreatedErr).NotTo(HaveOccurred())
					Expect(foundOrCreatedVolume).NotTo(BeNil())
				})
			})

			Context("when artifact does not exist in baggageclaim", func() {
				BeforeEach(func() {
					fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
				})

				It("returns an error", func() {
					Expect(foundOrCreatedErr).To(HaveOccurred())
					Expect(foundOrCreatedErr.Error()).To(ContainSubstring("artifact 'fake-handle' disappeared from worker 'fake-worker'"))
				})
			})
		})

		Context("when artifact does not exist in db", func() {
			BeforeEach(func() {
				fakeDBVolumeRepository.FindContainerVolumeReturns(nil, nil, nil)
				fakeLockFactory.AcquireReturns(fakeLock, true, nil)
			})

			It("acquires the lock", func() {
				Expect(fakeLockFactory.AcquireCallCount()).To(Equal(1))
			})

			It("creates artifact in creating state with parent artifact", func() {
				Expect(parentVolume.CreateChildForContainerCallCount()).To(Equal(1))
				actualContainer, actualMountPath := parentVolume.CreateChildForContainerArgsForCall(0)
				Expect(actualContainer).To(Equal(container))
				Expect(actualMountPath).To(Equal("some-mount-path"))
			})

			It("creates artifact in baggageclaim", func() {
				Expect(foundOrCreatedErr).NotTo(HaveOccurred())
				Expect(foundOrCreatedVolume).To(Equal(worker.NewArtifact(fakeBaggageclaimVolume, fakeCreatedVolume, volumeClient)))
				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
			})
		})
	})

	Describe("FindOrCreateVolumeForResourceCerts", func() {
		var (
			fakeBaggageclaimVolume *baggageclaimfakes.FakeVolume
			fakeCreatingVolume     *dbfakes.FakeCreatingVolume
			fakeCreatedVolume      *dbfakes.FakeCreatedVolume
			volume                 worker.Artifact
			found                  bool
			err                    error
		)

		BeforeEach(func() {
			fakeBaggageclaimVolume = new(baggageclaimfakes.FakeVolume)
			fakeBaggageclaimVolume.HandleReturns("fake-handle")

			fakeCreatedVolume = new(dbfakes.FakeCreatedVolume)
			fakeCreatedVolume.HandleReturns("fake-handle")

			fakeCreatingVolume = new(dbfakes.FakeCreatingVolume)
			fakeCreatingVolume.HandleReturns("fake-handle")
			fakeCreatingVolume.CreatedReturns(fakeCreatedVolume, nil)

			fakeLockFactory.AcquireReturns(fakeLock, true, nil)

			certPath := "/some/path/to/a/directory/of/certs"
			dbWorker.ResourceCertsReturns(&db.UsedWorkerResourceCerts{ID: 123}, true, nil)
			dbWorker.CertsPathReturns(&certPath)

			fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
		})

		JustBeforeEach(func() {
			volume, found, err = volumeClient.FindOrCreateVolumeForResourceCerts(testLogger)
		})

		Context("when the worker resource certs entry does not exist", func() {
			BeforeEach(func() {
				dbWorker.ResourceCertsReturns(nil, false, nil)
			})

			It("doesn't find a artifact", func() {
				Expect(found).To(BeFalse())
			})
		})

		Context("when a created artifact exists in the database", func() {
			BeforeEach(func() {
				fakeCreatedVolume.HandleReturns("created-handle")
				fakeDBVolumeRepository.FindResourceCertsVolumeReturns(nil, fakeCreatedVolume, nil)
			})

			It("looks up the artifact in baggageclaim", func() {
				Expect(fakeBaggageclaimClient.LookupVolumeCallCount()).To(Equal(1))
				_, handle := fakeBaggageclaimClient.LookupVolumeArgsForCall(0)
				Expect(handle).To(Equal("created-handle"))
			})

			It("finds a artifact", func() {
				Expect(found).To(BeTrue())
				Expect(volume).ToNot(BeNil())
			})
		})

		Context("when a creating artifact exists in the database", func() {
			BeforeEach(func() {
				fakeCreatingVolume.HandleReturns("creating-handle")
				fakeDBVolumeRepository.FindResourceCertsVolumeReturns(fakeCreatingVolume, nil, nil)
			})

			It("looks up the artifact in baggageclaim", func() {
				Expect(fakeBaggageclaimClient.LookupVolumeCallCount()).To(Equal(1))
				_, handle := fakeBaggageclaimClient.LookupVolumeArgsForCall(0)
				Expect(handle).To(Equal("creating-handle"))
			})

			It("marks the artifact as created in the db", func() {
				Expect(fakeCreatingVolume.CreatedCallCount()).To(Equal(1))
			})
		})

		Context("when the artifact doesn't exist on the worker", func() {
			BeforeEach(func() {
				fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
				fakeBaggageclaimClient.CreateVolumeReturns(fakeBaggageclaimVolume, nil)
				fakeDBVolumeRepository.CreateResourceCertsVolumeReturns(fakeCreatingVolume, nil)
			})

			Context("when the resource certs artifact doesn't exist in the db", func() {
				BeforeEach(func() {
					fakeDBVolumeRepository.FindResourceCertsVolumeReturns(nil, nil, nil)
				})

				It("creates the resource certs artifact in the db", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(fakeDBVolumeRepository.CreateResourceCertsVolumeCallCount()).To(Equal(1))
				})

				It("creates the artifact in baggageclaim", func() {
					Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
				})
			})

			Context("when a creating artifact exists in the database", func() {
				BeforeEach(func() {
					fakeCreatingVolume.HandleReturns("creating-handle")
					fakeDBVolumeRepository.FindResourceCertsVolumeReturns(fakeCreatingVolume, nil, nil)
				})

				It("looks up the artifact in baggageclaim", func() {
					Expect(fakeBaggageclaimClient.LookupVolumeCallCount()).To(Equal(1))
					_, handle := fakeBaggageclaimClient.LookupVolumeArgsForCall(0)
					Expect(handle).To(Equal("creating-handle"))
				})

				It("marks the artifact as created in the db", func() {
					Expect(fakeCreatingVolume.CreatedCallCount()).To(Equal(1))
				})
			})

			It("marks the artifact as created in the db", func() {
				Expect(fakeCreatingVolume.CreatedCallCount()).To(Equal(1))
			})

			It("creates the artifact in baggageclaim", func() {
				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
			})
		})
	})

	Describe("FindVolumeForTaskCache", func() {
		Context("when worker task cache does not exist", func() {
			BeforeEach(func() {
				fakeWorkerTaskCacheFactory.FindReturns(nil, false, nil)
			})

			It("returns false", func() {
				_, found, err := volumeClient.FindVolumeForTaskCache(testLogger, 123, 456, "some-step", "some-cache-path")
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when worker task cache exists", func() {
			var taskCache *db.UsedWorkerTaskCache
			BeforeEach(func() {
				taskCache = &db.UsedWorkerTaskCache{
					ID: 123,
				}

				fakeWorkerTaskCacheFactory.FindReturns(taskCache, true, nil)
			})

			Context("when task cache artifact does not exist in db", func() {
				BeforeEach(func() {
					fakeDBVolumeRepository.FindTaskCacheVolumeReturns(nil, nil, nil)
				})

				It("returns false", func() {
					_, found, err := volumeClient.FindVolumeForTaskCache(testLogger, 123, 456, "some-step", "some-cache-path")
					Expect(err).NotTo(HaveOccurred())
					Expect(found).To(BeFalse())
				})
			})

			Context("when task cache artifact exists in db", func() {
				var dbVolume db.CreatedVolume

				BeforeEach(func() {
					dbVolume = new(dbfakes.FakeCreatedVolume)
					fakeDBVolumeRepository.FindTaskCacheVolumeReturns(nil, dbVolume, nil)
				})

				Context("when task cache artifact does not exist in baggageclaim", func() {
					BeforeEach(func() {
						fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
					})

					It("returns false", func() {
						_, found, err := volumeClient.FindVolumeForTaskCache(testLogger, 123, 456, "some-step", "some-cache-path")
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeFalse())
					})
				})

				Context("when task cache artifact exists in baggageclaim", func() {
					var bcVolume *baggageclaimfakes.FakeVolume

					BeforeEach(func() {
						bcVolume = new(baggageclaimfakes.FakeVolume)
						fakeBaggageclaimClient.LookupVolumeReturns(bcVolume, true, nil)
					})

					It("returns artifact", func() {
						volume, found, err := volumeClient.FindVolumeForTaskCache(testLogger, 123, 456, "some-step", "some-cache-path")
						Expect(err).NotTo(HaveOccurred())
						Expect(found).To(BeTrue())

						Expect(volume).To(Equal(worker.NewArtifact(bcVolume, dbVolume, volumeClient)))
					})
				})
			})
		})
	})

	Describe("FindOrCreateVolumeForArtifact", func() {
		var err error
		var workerVolume worker.Artifact

		BeforeEach(func() {
			fakeLockFactory.AcquireReturns(nil, false, errors.New("nope"))
		})

		JustBeforeEach(func() {
			workerVolume, err = volumeClient.FindOrCreateVolumeForArtifact(testLogger, worker.VolumeSpec{}, 42, 123, "some-mount", db.VolumeTypeArtifact)
		})

		Context("when trying to create a new artifact", func() {

			Context("when creating the artifact in the db fails", func() {
				BeforeEach(func() {
					fakeDBVolumeRepository.CreateVolumeReturns(nil, errors.New("nope"))
				})

				It("errors", func() {
					Expect(err).To(HaveOccurred())
				})
			})

			Context("when creating the artifact in the db succeeds", func() {
				var fakeCreatingVolume *dbfakes.FakeCreatingVolume

				BeforeEach(func() {
					fakeCreatingVolume = new(dbfakes.FakeCreatingVolume)

					fakeDBVolumeRepository.CreateVolumeReturns(fakeCreatingVolume, nil)
				})

				Context("when it fails to acquire a lock", func() {
					BeforeEach(func() {
						fakeLockFactory.AcquireReturns(nil, false, errors.New("nope"))
					})

					It("errors", func() {
						Expect(err).To(HaveOccurred())
					})
				})

				Context("when it acquires the lock", func() {
					BeforeEach(func() {
						fakeLockFactory.AcquireReturns(fakeLock, true, nil)
					})

					It("creates artifact in baggageclaim", func() {
						Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
					})

					Context("when creating the artifact in baggage claim fails", func() {
						BeforeEach(func() {
							fakeBaggageclaimClient.CreateVolumeReturns(nil, errors.New("nope"))
						})

						It("errors", func() {
							Expect(err).To(HaveOccurred())
						})
					})

					Context("when creating the artifact in baggage claim succeeds", func() {
						var fakeBGVolume *baggageclaimfakes.FakeVolume

						BeforeEach(func() {
							fakeBGVolume = new(baggageclaimfakes.FakeVolume)
							fakeBaggageclaimClient.CreateVolumeReturns(fakeBGVolume, nil)
						})

						It("transitions the artifact to created in the db", func() {
							Expect(fakeCreatingVolume.CreatedCallCount()).To(Equal(1))
						})

						Context("when transitioning the build fails", func() {
							BeforeEach(func() {
								fakeCreatingVolume.CreatedReturns(nil, errors.New("nope"))
							})

							It("errors", func() {
								Expect(err).To(HaveOccurred())
							})
						})

						Context("when transitioning the build succeeds", func() {
							var fakeCreatedVolume *dbfakes.FakeCreatedVolume

							BeforeEach(func() {
								fakeCreatedVolume = new(dbfakes.FakeCreatedVolume)
								fakeCreatingVolume.CreatedReturns(fakeCreatedVolume, nil)
							})

							It("returns a new artifact with the bg artifact and created artifact", func() {
								Expect(err).NotTo(HaveOccurred())
								Expect(workerVolume).To(Equal(worker.NewArtifact(fakeBGVolume, fakeCreatedVolume, volumeClient)))
							})
						})
					})
				})
			})
		})
	})

	Describe("LookupVolume", func() {
		var handle string

		var found bool
		var lookupErr error

		BeforeEach(func() {
			handle = "some-handle"

			fakeCreatedVolume := new(dbfakes.FakeCreatedVolume)
			fakeDBVolumeRepository.FindCreatedVolumeReturns(fakeCreatedVolume, true, nil)
		})

		JustBeforeEach(func() {
			_, found, lookupErr = worker.NewVolumeClient(
				fakeBaggageclaimClient,
				dbWorker,
				fakeClock,

				fakeLockFactory,
				fakeDBVolumeRepository,
				fakeWorkerBaseResourceTypeFactory,
				fakeWorkerTaskCacheFactory,
			).LookupVolume(testLogger, handle)
		})

		Context("when the artifact can be found on baggageclaim", func() {
			var fakeBaggageclaimVolume *baggageclaimfakes.FakeVolume

			BeforeEach(func() {
				fakeBaggageclaimVolume = new(baggageclaimfakes.FakeVolume)
				fakeBaggageclaimVolume.HandleReturns(handle)
				fakeBaggageclaimClient.LookupVolumeReturns(fakeBaggageclaimVolume, true, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).ToNot(HaveOccurred())
			})

			It("looks up the artifact via BaggageClaim", func() {
				Expect(fakeBaggageclaimClient.LookupVolumeCallCount()).To(Equal(1))

				_, lookedUpHandle := fakeBaggageclaimClient.LookupVolumeArgsForCall(0)
				Expect(lookedUpHandle).To(Equal(handle))
			})
		})

		Context("when the artifact cannot be found on baggageclaim", func() {
			BeforeEach(func() {
				fakeBaggageclaimClient.LookupVolumeReturns(nil, false, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).ToNot(HaveOccurred())
			})

			It("returns false", func() {
				Expect(found).To(BeFalse())
			})
		})

		Context("when the artifact cannot be found in database", func() {
			BeforeEach(func() {
				fakeDBVolumeRepository.FindCreatedVolumeReturns(nil, false, nil)
			})

			It("succeeds", func() {
				Expect(lookupErr).ToNot(HaveOccurred())
			})

			It("returns false", func() {
				Expect(found).To(BeFalse())
			})
		})

		Context("when looking up the artifact fails", func() {
			disaster := errors.New("nope")

			BeforeEach(func() {
				fakeBaggageclaimClient.LookupVolumeReturns(nil, false, disaster)
			})

			It("returns the error", func() {
				Expect(lookupErr).To(Equal(disaster))
			})
		})
	})

	Describe("EnsureLocalCopyOfArtifact", func() {
		var (
			fakeDbArtifact *dbfakes.FakeWorkerArtifact
			fakeDbVolume   *dbfakes.FakeCreatedVolume
			fakeBcVolume   *baggageclaimfakes.FakeVolume
			artifact       *workerfakes.FakeArtifact

			fakeCreatingContainer *dbfakes.FakeCreatingContainer
			err                   error
		)

		BeforeEach(func() {
			fakeDbArtifact = new(dbfakes.FakeWorkerArtifact)
			fakeDbVolume = new(dbfakes.FakeCreatedVolume)
			fakeBcVolume = new(baggageclaimfakes.FakeVolume)

			fakeCreatingContainer = new(dbfakes.FakeCreatingContainer)
			fakeCreatingContainer.IDReturns(123)

			artifact = new(workerfakes.FakeArtifact)
		})
		JustBeforeEach(func() {
			err = volumeClient.EnsureLocalCopyOfArtifact(testLogger, artifact, fakeCreatingContainer)
			Expect(err).ToNot(HaveOccurred())
		})

		Context("artifact volume available on current worker", func() {
			BeforeEach(func() {
				fakeDbVolume.WorkerNameReturns(dbWorker.Name())
				// artifact volume should exist, container-specific volume should not exist
				fakeDBVolumeRepository.FindArtifactVolumeReturns(nil, fakeDbVolume, nil)
				fakeDBVolumeRepository.FindContainerVolumeReturns(nil, nil, nil)
			})
			It("creates a COW volume of the artifact for the container", func() {
				Expect(artifact.CreateChildForContainerCallCount()).To(Equal(1))
				creatingContainer, _ := artifact.CreateChildForContainerArgsForCall(0)
				Expect(creatingContainer).To(Equal(fakeCreatingContainer))

				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
				_, _, strategy := fakeBaggageclaimClient.CreateVolumeArgsForCall(0)
				Expect(strategy).To(Equal(baggageclaim.COWStrategy{}))
			})
		})
		Context("artifact volume available on other worker", func() {
			BeforeEach(func() {
				fakeDbVolume.WorkerNameReturns("other-worker")
				// artifact volume should exist, container-specific volume should not exist
				fakeDBVolumeRepository.FindArtifactVolumeReturns(nil, fakeDbVolume, nil)
				fakeDBVolumeRepository.FindContainerVolumeReturns(nil, nil, nil)
			})
			It("create a new empty volume for the container", func() {
				Expect(fakeDBVolumeRepository.CreateContainerVolumeCallCount()).To(Equal(1))
				// TODO: CreateContainerVolume also takes a mountpath. Should we test it?
				_, workerName, creatingContainer, _ := fakeDBVolumeRepository.CreateContainerVolumeArgsForCall(0)
				Expect(workerName).To(Equal(dbWorker.Name()))
				Expect(creatingContainer).To(Equal(fakeCreatingContainer))

				Expect(fakeBaggageclaimClient.CreateVolumeCallCount()).To(Equal(1))
				_, _, strategy := fakeBaggageclaimClient.CreateVolumeArgsForCall(0)
				Expect(strategy).To(Equal(baggageclaim.EmptyStrategy{}))
			})

			It("streams in the artifact contents into the new volume", func() {
				Expect(artifact.StreamOutCallCount()).To(Equal(1))

			})

			It("does not attach the new volume to the artifact", func() {

			})
		})
		Context("artifact volume not available on any worker", func() {
			It("creates a new blank volume for the artifact", func() {

			})
			It("attaches the new volume to the artifact", func() {

			})
		})
	})
})
