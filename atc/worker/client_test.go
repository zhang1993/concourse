package worker_test

import (
	"errors"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Client", func() {
	var (
		logger       *lagertest.TestLogger
		fakePool     *workerfakes.FakePool
		fakeProvider *workerfakes.FakeWorkerProvider
		client       worker.Client
	)

	BeforeEach(func() {
		logger = lagertest.NewTestLogger("test")
		fakePool = new(workerfakes.FakePool)
		fakeProvider = new(workerfakes.FakeWorkerProvider)

		client = worker.NewClient(fakePool, fakeProvider)
	})

	Describe("FindContainer", func() {
		var (
			foundContainer worker.Container
			found          bool
			findErr        error
		)

		JustBeforeEach(func() {
			foundContainer, found, findErr = client.FindContainer(
				logger,
				4567,
				"some-handle",
			)
		})

		Context("when looking up the worker errors", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForContainerReturns(nil, false, errors.New("nope"))
			})

			It("errors", func() {
				Expect(findErr).To(HaveOccurred())
			})
		})

		Context("when worker is not found", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForContainerReturns(nil, false, nil)
			})

			It("returns not found", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when a worker is found with the container", func() {
			var fakeWorker *workerfakes.FakeWorker
			var fakeContainer *workerfakes.FakeContainer

			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeProvider.FindWorkerForContainerReturns(fakeWorker, true, nil)

				fakeContainer = new(workerfakes.FakeContainer)
				fakeWorker.FindContainerByHandleReturns(fakeContainer, true, nil)
			})

			It("succeeds", func() {
				Expect(found).To(BeTrue())
				Expect(findErr).NotTo(HaveOccurred())
			})

			It("returns the created container", func() {
				Expect(foundContainer).To(Equal(fakeContainer))
			})
		})
	})

	Describe("FindVolume", func() {
		var (
			foundVolume worker.Volume
			found       bool
			findErr     error
		)

		JustBeforeEach(func() {
			foundVolume, found, findErr = client.FindVolume(
				logger,
				4567,
				"some-handle",
			)
		})

		Context("when looking up the worker errors", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForVolumeReturns(nil, false, errors.New("nope"))
			})

			It("errors", func() {
				Expect(findErr).To(HaveOccurred())
			})
		})

		Context("when worker is not found", func() {
			BeforeEach(func() {
				fakeProvider.FindWorkerForVolumeReturns(nil, false, nil)
			})

			It("returns not found", func() {
				Expect(findErr).NotTo(HaveOccurred())
				Expect(found).To(BeFalse())
			})
		})

		Context("when a worker is found with the volume", func() {
			var fakeWorker *workerfakes.FakeWorker
			var fakeVolume *workerfakes.FakeVolume

			BeforeEach(func() {
				fakeWorker = new(workerfakes.FakeWorker)
				fakeProvider.FindWorkerForVolumeReturns(fakeWorker, true, nil)

				fakeVolume = new(workerfakes.FakeVolume)
				fakeWorker.LookupVolumeReturns(fakeVolume, true, nil)
			})

			It("succeeds", func() {
				Expect(found).To(BeTrue())
				Expect(findErr).NotTo(HaveOccurred())
			})

			It("returns the volume", func() {
				Expect(foundVolume).To(Equal(fakeVolume))
			})
		})
	})

	// Describe("CreateArtifact", func() {
	// 	var (
	// 		fakeWorker     *workerfakes.FakeWorker
	// 		fakeDBArtifact *dbfakes.FakeWorkerArtifact
	// 		err            error
	// 		artifact       worker.Artifact
	// 		fakeArtifactProvider *dbfakes.FakeArtifactProvider
	// 	)
	//
	// 	JustBeforeEach(func() {
	// 		fakeDBArtifact = new(dbfakes.FakeWorkerArtifact)
	// 		fakeWorker = new(workerfakes.FakeWorker)
	// 		fakeWorker.NameReturns("workerA")
	// 		fakeWorker.SatisfiesReturns(true)
	//
	// 		fakeProvider.RunningWorkersReturns([]worker.Worker{fakeWorker}, nil)
	//
	// 		fakeDBArtifact.IDReturns(1)
	// 		fakeDBArtifact.NameReturns("some-artifact")
	// 		fakeArtifactProvider.CreateArtifactReturns(fakeDBArtifact, nil)
	//
	// 		artifact, err = pool.CreateArtifact(logger, 1, "some-artifact")
	// 		Expect(err).ToNot(HaveOccurred())
	// 		Expect(artifact).ToNot(BeNil())
	// 	})
	//
	// 	It("creates an uninitialized artifact", func() {
	// 		Expect(artifact.Initialized()).To(BeFalse())
	// 	})
	//
	// 	Context("when the worker can be found", func() {
	// 		It("creates the volume on the worker", func() {
	// 			Expect(err).ToNot(HaveOccurred())
	// 			Expect(fakeWorker.CreateVolumeForArtifactCallCount()).To(Equal(1))
	// 			l, teamID, artifactID := fakeWorker.CreateVolumeForArtifactArgsForCall(0)
	// 			Expect(l).To(Equal(logger))
	// 			Expect(teamID).To(Equal(1))
	// 			Expect(artifactID).To(Equal(1))
	// 		})
	// 	})
	// })

})
