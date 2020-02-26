package tsa_test

import (
	"errors"
	"github.com/concourse/concourse/tsa"
	"github.com/concourse/concourse/tsa/tsafakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"time"
)

var _ = Describe("Timeout Conn Test", func() {
	var (
		fakeConn tsafakes.FakeConn
		conn     tsa.IdleTimeoutConn
		timeout  time.Duration
	)

	BeforeEach(func() {
		timeout = time.Millisecond * 200
		fakeConn = tsafakes.FakeConn{}
		conn = tsa.IdleTimeoutConn{
			&fakeConn,
			timeout,
		}
	})

	Context("timeoutConn.Read", func() {
		var (
			givenBytes        []byte
			someErr           error
			someNumberOfBytes int
		)
		BeforeEach(func() {
			givenBytes = make([]byte,100)
			someErr = errors.New("some foo err")
			someNumberOfBytes = 42
			fakeConn.ReadReturns(someNumberOfBytes, someErr)
		})
		It("Passes the byte array to the underlying net.Conn and returns appropriate values", func() {
			actualNumberOfBytes, actualErr := conn.Read(givenBytes)
			Expect(actualErr).To(Equal(someErr))
			Expect(actualNumberOfBytes).To(Equal(someNumberOfBytes))

			actualByteArray := fakeConn.ReadArgsForCall(0)
			Expect(actualByteArray).To(Equal(givenBytes))
		})
	})

	Context("timeoutConn.Write", func() {
		var (
			bytesToWrite        []byte
			someErr           error
			someNumberOfBytes int
		)
		BeforeEach(func() {
			bytesToWrite = make([]byte,100)
			someErr = errors.New("some foo err")
			someNumberOfBytes = 42
			fakeConn.WriteReturns(someNumberOfBytes, someErr)
		})
		It("Passes the byte array to the underlying net.Conn and returns appropriate values", func() {
			actualNumberOfBytes, actualErr := conn.Write(bytesToWrite)
			Expect(actualErr).To(Equal(someErr))
			Expect(actualNumberOfBytes).To(Equal(someNumberOfBytes))

			actualByteArray := fakeConn.WriteArgsForCall(0)
			Expect(actualByteArray).To(Equal(bytesToWrite))
		})
	})

	Context("When a Read is successful", func() {
		BeforeEach(func() {
			fakeConn.ReadReturns(0, nil)
		})
		It("Updates the deadline", func() {
			_, err := conn.Read([]byte{})
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeConn.SetDeadlineCallCount()).To(Equal(1))
			deadline := fakeConn.SetDeadlineArgsForCall(0)
			Expect(time.Now().Before(deadline)).To(BeTrue())
		})
	})

	Context("When a Read is NOT successful", func() {
		var (
			someErr error
		)
		BeforeEach(func() {
			someErr = errors.New("some foo error")
			fakeConn.ReadReturns(0, someErr)
		})
		It("does NOT update the deadline", func() {
			_, err := conn.Read([]byte{})
			Expect(err).To(Equal(someErr))

			Expect(fakeConn.SetDeadlineCallCount()).To(Equal(0))
		})
	})

	Context("When a Write is successful", func() {
		BeforeEach(func() {
			fakeConn.WriteReturns(0, nil)
		})
		It("Updates the deadline", func() {
			_, err := conn.Write([]byte("some foo"))
			Expect(err).ToNot(HaveOccurred())

			Expect(fakeConn.SetDeadlineCallCount()).To(Equal(1))
			deadline := fakeConn.SetDeadlineArgsForCall(0)
			Expect(time.Now().Before(deadline)).To(BeTrue())
		})
	})

	Context("When a Write is NOT successful", func() {
		var (
			someErr error
		)
		BeforeEach(func() {
			someErr = errors.New("some foo error")
			fakeConn.WriteReturns(0, someErr)
		})
		It("does NOT update the deadline", func() {
			_, err := conn.Write([]byte("some foo"))
			Expect(err).To(Equal(someErr))

			Expect(fakeConn.SetDeadlineCallCount()).To(Equal(0))
		})
	})
})
