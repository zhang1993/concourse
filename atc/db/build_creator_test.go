package db_test

import (
	"fmt"
	"github.com/concourse/concourse/atc"
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/db/dbfakes"
	"github.com/concourse/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"strconv"
)

var _ = Describe("BuildCreator", func() {
	var (
		fakeEventProcessor *dbfakes.FakeEventProcessor
	)

	BeforeEach(func() {
		fakeEventProcessor = new(dbfakes.FakeEventProcessor)
		buildCreator = db.NewBuildCreator(dbConn, lockFactory, fakeEventProcessor)
	})

	Describe("CreateStartedBuild", func() {
		var (
			plan         atc.Plan
			startedBuild db.Build
			err          error
		)

		BeforeEach(func() {
			plan = atc.Plan{
				ID: atc.PlanID("56"),
				Get: &atc.GetPlan{
					Type:     "some-type",
					Name:     "some-name",
					Resource: "some-resource",
					Source:   atc.Source{"some": "source"},
					Params:   atc.Params{"some": "params"},
					Version:  &atc.Version{"some": "version"},
					Tags:     atc.Tags{"some-tags"},
					VersionedResourceTypes: atc.VersionedResourceTypes{
						{
							ResourceType: atc.ResourceType{
								Name:       "some-name",
								Source:     atc.Source{"some": "source"},
								Type:       "some-type",
								Privileged: true,
								Tags:       atc.Tags{"some-tags"},
							},
							Version: atc.Version{"some-resource-type": "version"},
						},
					},
				},
			}

			startedBuild, err = buildCreator.CreateStartedBuild(defaultPipeline.TeamID(), defaultPipeline.ID(), plan)
			Expect(err).ToNot(HaveOccurred())
		})

		It("can create started builds with plans", func() {
			Expect(startedBuild.ID()).ToNot(BeZero())
			Expect(startedBuild.JobName()).To(BeZero())
			Expect(startedBuild.PipelineName()).To(Equal(defaultPipeline.Name()))
			Expect(startedBuild.Name()).To(Equal(strconv.Itoa(startedBuild.ID())))
			Expect(startedBuild.TeamName()).To(Equal(defaultPipeline.TeamName()))
			Expect(startedBuild.Status()).To(Equal(db.BuildStatusStarted))
		})

		It("saves the public plan", func() {
			found, err := startedBuild.Reload()
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeTrue())
			Expect(startedBuild.PublicPlan()).To(Equal(plan.Public()))
		})

		It("initializes the event processor with the build", func() {
			Expect(fakeEventProcessor.InitializeCallCount()).To(Equal(1))
			Expect(fakeEventProcessor.InitializeArgsForCall(0)).To(BeIdenticalTo(startedBuild))
		})

		It("creates Start event", func() {
			Expect(fakeEventProcessor.ProcessCallCount()).To(Equal(1))
			build, evt := fakeEventProcessor.ProcessArgsForCall(0)
			Expect(build).To(BeIdenticalTo(startedBuild))
			Expect(evt).To(Equal(event.Status{
				Status: atc.StatusStarted,
				Time:   startedBuild.StartTime().Unix(),
			}))
		})
	})

	Describe("CreateBuild", func() {
		var build db.Build

		BeforeEach(func() {
			var err error
			build, err = buildCreator.CreateBuild(defaultJob)
			Expect(err).ToNot(HaveOccurred())
		})

		It("initializes the event processor with the build", func() {
			Expect(fakeEventProcessor.InitializeCallCount()).To(Equal(1))
			Expect(fakeEventProcessor.InitializeArgsForCall(0)).To(BeIdenticalTo(build))
		})

		It("increments the build number", func() {
			Expect(build.Name()).To(Equal("1"))

			secondBuild, err := buildCreator.CreateBuild(defaultJob)
			Expect(err).ToNot(HaveOccurred())
			Expect(secondBuild.Name()).To(Equal("2"))
		})
	})

	Describe("RerunBuild", func() {
		var firstBuild db.Build
		var rerunErr error
		var rerunBuild db.Build
		var buildToRerun db.Build

		JustBeforeEach(func() {
			rerunBuild, rerunErr = buildCreator.RerunBuild(defaultJob, buildToRerun)
		})

		Context("when the first build exists", func() {
			BeforeEach(func() {
				var err error
				firstBuild, err = buildCreator.CreateBuild(defaultJob)
				Expect(err).NotTo(HaveOccurred())

				buildToRerun = firstBuild
			})

			It("finds the build", func() {
				Expect(rerunErr).ToNot(HaveOccurred())
				Expect(rerunBuild.Name()).To(Equal(fmt.Sprintf("%s.1", firstBuild.Name())))
				Expect(rerunBuild.RerunNumber()).To(Equal(1))

				build, found, err := defaultJob.Build(rerunBuild.Name())
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())
				Expect(build.ID()).To(Equal(rerunBuild.ID()))
				Expect(build.Status()).To(Equal(rerunBuild.Status()))
			})

			It("requests schedule on the job", func() {
				requestedSchedule := defaultJob.ScheduleRequestedTime()

				_, err := buildCreator.RerunBuild(defaultJob, buildToRerun)
				Expect(err).NotTo(HaveOccurred())

				found, err := defaultJob.Reload()
				Expect(err).NotTo(HaveOccurred())
				Expect(found).To(BeTrue())

				Expect(defaultJob.ScheduleRequestedTime()).Should(BeTemporally(">", requestedSchedule))
			})

			It("initializes the event processor with the build", func() {
				Expect(fakeEventProcessor.InitializeCallCount()).To(Equal(2))
				Expect(fakeEventProcessor.InitializeArgsForCall(1)).To(BeIdenticalTo(rerunBuild))
			})

			Context("when there is an existing rerun build", func() {
				var rerun1 db.Build

				BeforeEach(func() {
					var err error
					rerun1, err = buildCreator.RerunBuild(defaultJob, buildToRerun)
					Expect(err).ToNot(HaveOccurred())
					Expect(rerun1.Name()).To(Equal(fmt.Sprintf("%s.1", firstBuild.Name())))
					Expect(rerun1.RerunNumber()).To(Equal(1))
				})

				It("increments the rerun build number", func() {
					Expect(rerunErr).ToNot(HaveOccurred())
					Expect(rerunBuild.Name()).To(Equal(fmt.Sprintf("%s.2", firstBuild.Name())))
					Expect(rerunBuild.RerunNumber()).To(Equal(rerun1.RerunNumber() + 1))
				})
			})

			Context("when we try to rerun a rerun build", func() {
				var rerun1 db.Build

				BeforeEach(func() {
					var err error
					rerun1, err = buildCreator.RerunBuild(defaultJob, buildToRerun)
					Expect(err).ToNot(HaveOccurred())
					Expect(rerun1.Name()).To(Equal(fmt.Sprintf("%s.1", firstBuild.Name())))
					Expect(rerun1.RerunNumber()).To(Equal(1))

					buildToRerun = rerun1
				})

				It("keeps the name of original build and increments the rerun build number", func() {
					Expect(rerunErr).ToNot(HaveOccurred())
					Expect(rerunBuild.Name()).To(Equal(fmt.Sprintf("%s.2", firstBuild.Name())))
					Expect(rerunBuild.RerunNumber()).To(Equal(rerun1.RerunNumber() + 1))
				})
			})
		})
	})

	Describe("EnsurePendingBuildExists", func() {
		Context("when only a started build exists", func() {
			It("creates a build and updates the next build for the job", func() {
				err := buildCreator.EnsurePendingBuildExists(defaultJob)
				Expect(err).NotTo(HaveOccurred())

				pendingBuilds, err := defaultJob.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuilds).To(HaveLen(1))

				_, nextBuild, err := defaultJob.FinishedAndNextBuild()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuilds[0].ID()).To(Equal(nextBuild.ID()))
			})

			It("initializes the event processor with the build", func() {
				err := buildCreator.EnsurePendingBuildExists(defaultJob)
				Expect(err).NotTo(HaveOccurred())

				pendingBuilds, err := defaultJob.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(pendingBuilds).To(HaveLen(1))

				Expect(fakeEventProcessor.InitializeCallCount()).To(Equal(1))
				Expect(fakeEventProcessor.InitializeArgsForCall(0)).To(Equal(pendingBuilds[0]))
			})

			It("doesn't create another build the second time it's called", func() {
				err := buildCreator.EnsurePendingBuildExists(defaultJob)
				Expect(err).NotTo(HaveOccurred())

				err = buildCreator.EnsurePendingBuildExists(defaultJob)
				Expect(err).NotTo(HaveOccurred())

				builds2, err := defaultJob.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(builds2).To(HaveLen(1))

				started, err := builds2[0].Start(atc.Plan{}, fakeEventProcessor)
				Expect(err).NotTo(HaveOccurred())
				Expect(started).To(BeTrue())

				builds2, err = defaultJob.GetPendingBuilds()
				Expect(err).NotTo(HaveOccurred())
				Expect(builds2).To(HaveLen(0))
			})
		})
	})
})