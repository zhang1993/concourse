package db_test

import (
	"github.com/concourse/concourse/atc/db"
	"github.com/concourse/concourse/atc/event"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("BuildEventStore", func() {
	Describe("Process", func() {
		It("saves and propagates events correctly", func() {
			build, err := buildCreator.CreateBuild(defaultJob)
			Expect(err).NotTo(HaveOccurred())

			By("allowing you to subscribe when no events have yet occurred")
			events, err := buildEventStore.Events(build, 0)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(events)

			By("saving them in order")
			err = buildEventStore.Process(build, event.Log{
				Payload: "some ",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(events.Next()).To(Equal(envelope(event.Log{
				Payload: "some ",
			})))

			err = buildEventStore.Process(build, event.Log{
				Payload: "log",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(events.Next()).To(Equal(envelope(event.Log{
				Payload: "log",
			})))

			By("allowing you to subscribe from an offset")
			eventsFrom1, err := buildEventStore.Events(build, 1)
			Expect(err).NotTo(HaveOccurred())

			defer db.Close(eventsFrom1)

			Expect(eventsFrom1.Next()).To(Equal(envelope(event.Log{
				Payload: "log",
			})))

			By("notifying those waiting on events as soon as they're saved")
			nextEvent := make(chan event.Envelope)
			nextErr := make(chan error)

			go func() {
				event, err := events.Next()
				if err != nil {
					nextErr <- err
				} else {
					nextEvent <- event
				}
			}()

			Consistently(nextEvent).ShouldNot(Receive())
			Consistently(nextErr).ShouldNot(Receive())

			err = buildEventStore.Process(build, event.Log{
				Payload: "log 2",
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(nextEvent).Should(Receive(Equal(envelope(event.Log{
				Payload: "log 2",
			}))))

			By("returning ErrBuildEventStreamClosed for Next calls after Close")
			events3, err := buildEventStore.Events(build, 0)
			Expect(err).NotTo(HaveOccurred())

			err = events3.Close()
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				_, err := events3.Next()
				return err
			}).Should(Equal(db.ErrBuildEventStreamClosed))
		})
	})

	Describe("Delete", func() {
		It("deletes all build logs corresponding to the given build ids", func() {
			build1DB, err := buildCreator.CreateBuild(defaultJob)
			Expect(err).ToNot(HaveOccurred())

			err = buildEventStore.Process(build1DB, event.Log{
				Payload: "log 1",
			})
			Expect(err).ToNot(HaveOccurred())

			build2DB, err := buildCreator.CreateBuild(defaultJob)
			Expect(err).ToNot(HaveOccurred())

			err = buildEventStore.Process(build2DB, event.Log{
				Payload: "log 2",
			})
			Expect(err).ToNot(HaveOccurred())

			build3DB, err := buildCreator.CreateBuild(defaultJob)
			Expect(err).ToNot(HaveOccurred())

			err = finishBuild(build3DB, db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = finishBuild(build1DB, db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			err = finishBuild(build2DB, db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			build4DB, err := buildCreator.CreateBuild(defaultJob)
			Expect(err).ToNot(HaveOccurred())

			By("doing nothing if there are no build ids passed in")
			err = buildEventStore.Delete()
			Expect(err).ToNot(HaveOccurred())

			By("not returning an error")
			err = buildEventStore.Delete(build3DB.ID(), build4DB.ID(), build1DB.ID())
			Expect(err).ToNot(HaveOccurred())

			err = finishBuild(build4DB, db.BuildStatusSucceeded)
			Expect(err).ToNot(HaveOccurred())

			By("deleting events for build 1")
			events1, err := buildEventStore.Events(build1DB, 0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events1)

			_, err = events1.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("preserving events for build 2")
			events2, err := buildEventStore.Events(build2DB, 0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events2)

			build2Event1, err := events2.Next()
			Expect(err).ToNot(HaveOccurred())
			Expect(build2Event1).To(Equal(envelope(event.Log{
				Payload: "log 2",
			})))

			_, err = events2.Next() // finish event
			Expect(err).ToNot(HaveOccurred())

			_, err = events2.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("deleting events for build 3")
			events3, err := buildEventStore.Events(build3DB, 0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events3)

			_, err = events3.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("being unflapped by build 4, which had no events at the time")
			events4, err := buildEventStore.Events(build4DB, 0)
			Expect(err).ToNot(HaveOccurred())
			defer db.Close(events4)

			_, err = events4.Next() // finish event
			Expect(err).ToNot(HaveOccurred())

			_, err = events4.Next()
			Expect(err).To(Equal(db.ErrEndOfBuildEventStream))

			By("updating ReapTime for the affected builds")
			found, err := build1DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build1DB.ReapTime()).To(BeTemporally(">", build1DB.EndTime()))

			found, err = build2DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build2DB.ReapTime()).To(BeZero())

			found, err = build3DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			Expect(build3DB.ReapTime()).To(Equal(build1DB.ReapTime()))

			found, err = build4DB.Reload()
			Expect(err).ToNot(HaveOccurred())
			Expect(found).To(BeTrue())

			// Not required behavior, just a sanity check for what I think will happen
			Expect(build4DB.ReapTime()).To(Equal(build1DB.ReapTime()))
		})
	})
})
