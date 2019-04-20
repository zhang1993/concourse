package worker_test

import (
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/worker"
	"github.com/concourse/concourse/atc/worker/workerfakes"
)

var _ = Describe("ArtifactManager", func() {

	var (
		fakeVolumeClient      *workerfakes.FakeVolumeClient
		fakeArtifactLifecycle *dbfakes.FakeWorkerArtifactLifecycle
		fakeDBArtifact        *dbfakes.FakeWorkerArtifact
		fakeDBResourceCache   *dbfakes.FakeUsedResourceCache
		fakeVolume            *workerfakes.FakeVolume

		artifactManager worker.ArtifactManager
		testLogger      lagertest.TestLogger
	)

	BeforeEach(func() {
		fakeDBArtifact = new(dbfakes.FakeWorkerArtifact)
		fakeDBArtifact.IDReturns(5)
		fakeVolume = new(workerfakes.FakeVolume)
		fakeVolume.HandleReturns("some-handle")

		fakeArtifactLifecycle = new(dbfakes.FakeWorkerArtifactLifecycle)
		fakeVolumeClient = new(workerfakes.FakeVolumeClient)
		fakeVolumeClient.FindVolumeForArtifactReturns(fakeVolume, nil)
		artifactManager = worker.NewArtifactManager(fakeArtifactLifecycle, fakeVolumeClient)
	})

	Describe("FindOrCreateArtifact", func() {
		BeforeEach(func() {
			fakeDBResourceCache = new(dbfakes.FakeUsedResourceCache)
			fakeDBResourceCache.IDReturns(7)
		})
		Context("artifact does exist", func() {

			It("finds the artifact", func() {

				fakeArtifactLifecycle.FindArtifactForResourceCacheReturns(fakeDBArtifact, nil)

				artifact, err := artifactManager.FindOrCreateArtifact(testLogger, fakeDBResourceCache.ID(), db.ArtifactTypeResourceCache)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeArtifactLifecycle.FindArtifactForResourceCacheCallCount()).To(Equal(1))
				Expect(fakeArtifactLifecycle.CreateArtifactCallCount()).To(Equal(0))
				Expect(artifact.DBArtifact()).To(Equal(fakeDBArtifact))
			})

		})
		Context("artifact does not exist", func() {

			It("creates the artifact", func() {
				fakeArtifactLifecycle.FindArtifactForResourceCacheReturns(nil, nil)
				fakeArtifactLifecycle.CreateArtifactReturns(fakeDBArtifact, nil)

				artifact, err := artifactManager.FindOrCreateArtifact(testLogger, fakeDBResourceCache.ID(), db.ArtifactTypeResourceCache)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeArtifactLifecycle.FindArtifactForResourceCacheCallCount()).To(Equal(1))
				Expect(fakeArtifactLifecycle.CreateArtifactCallCount()).To(Equal(1))
				Expect(artifact.DBArtifact()).To(Equal(fakeDBArtifact))
			})
		})
	})

	// Describe("FindArtifactForResourceCache", func() {
	// 	var (
	// 		artifact worker.Artifact
	// 		found    bool
	// 		err      error
	// 	)
	// 	JustBeforeEach(func() {
	// 		artifact, found, err = artifactManager.FindArtifactForResourceCache(testLogger, fakeDBResourceCache.ID())
	// 	})
	//
	// 	BeforeEach(func() {
	// 		fakeDBResourceCache = new(dbfakes.FakeUsedResourceCache)
	// 		fakeDBResourceCache.IDReturns(7)
	// 	})
	//
	// 	Context("resource cache does not exist", func() {
	// 		BeforeEach(func() {
	// 			fakeArtifactLifecycle.FindArtifactForResourceCacheReturns(nil, nil)
	// 		})
	//
	// 		It("returns false", func() {
	// 			Expect(err).ToNot(HaveOccurred())
	// 			Expect(found).To(BeFalse())
	// 			Expect(artifact).To(BeNil())
	// 		})
	// 	})
	//
	// 	Context("resource cache exists", func() {
	// 		BeforeEach(func() {
	// 			fakeArtifactLifecycle.FindArtifactForResourceCacheReturns(fakeDBArtifact, nil)
	// 		})
	//
	// 		Context("volume does not exist", func() {
	// 			BeforeEach(func() {
	// 				fakeVolumeClient.FindVolumeForArtifactReturns(nil, nil)
	// 			})
	// 			It("returns false", func() {
	// 				Expect(err).ToNot(HaveOccurred())
	// 				Expect(found).To(BeFalse())
	// 				Expect(artifact).To(BeNil())
	// 			})
	// 		})
	//
	// 		Context("volume exists", func() {
	// 			BeforeEach(func() {
	// 				fakeVolumeClient.FindVolumeForArtifactReturns(fakeVolume, nil)
	// 			})
	//
	// 			It("returns an Artifact corresponding to the resource cache", func() {
	// 				Expect(artifact).ToNot(BeNil())
	// 				Expect(err).ToNot(HaveOccurred())
	// 				Expect(found).To(BeTrue())
	//
	// 				Expect(fakeArtifactLifecycle.FindArtifactForResourceCacheCallCount()).To(Equal(1))
	// 				_, rcID := fakeArtifactLifecycle.FindArtifactForResourceCacheArgsForCall(0)
	// 				Expect(rcID).To(Equal(fakeDBResourceCache.ID()))
	// 				Expect(artifact.Volume().Handle()).To(Equal("some-handle"))
	// 			})
	// 		})
	// 	})
	// })
	Describe("FindArtifactForTaskCache", func() {

		JustBeforeEach(func() {
			// fakeTeamID := 1
			// fakeJobID := 25
			// fakeStepName := ""
			// fakePath string

		})

		It("returns an Artifact corresponding to the resource cache", func() {

		})
	})
	Describe("CertsArtifact", func() {

		JustBeforeEach(func() {
		})

		It("returns an Artifact corresponding to the resource cache", func() {

		})
	})
	Describe("CreateVolumeForArtifact", func() {

		JustBeforeEach(func() {
		})

		It("returns an Artifact corresponding to the resource cache", func() {

		})
	})
})
