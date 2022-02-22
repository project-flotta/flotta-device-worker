package logs_test

import (
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/logs"
)

var _ = Describe("FifoLog", func() {

	Context("Write", func() {

		It("No entry", func() {

			// given
			fifolog := logs.NewFIFOLog(10)

			// when
			err := fifolog.Write(nil)

			// then
			Expect(err).To(HaveOccurred())
			Expect(fifolog.CurrentSize()).To(Equal(0))
		})

		It("Size is bigger than expected", func() {

			// given
			fifolog := logs.NewFIFOLog(1)

			err := fifolog.Write(logs.NewLogEntry([]byte("123"), "test"))
			Expect(err).NotTo(HaveOccurred())
			Expect(fifolog.CurrentSize()).To(Equal(3))

			// when
			err = fifolog.Write(logs.NewLogEntry([]byte("a"), "test"))

			// then
			Expect(err).To(HaveOccurred())
			Expect(fifolog.CurrentSize()).To(Equal(3))
		})

	})

	Context("Readline", func() {
		It("Reads and got deleted", func() {
			// given
			fifolog := logs.NewFIFOLog(1000)
			for i := 0; i < 10; i++ {
				err := fifolog.Write(logs.NewLogEntry([]byte(fmt.Sprintf("%d", i)), "test"))
				Expect(err).NotTo(HaveOccurred())
			}

			// When
			for i := 0; i < 10; i++ {
				entry, err := fifolog.ReadLine()
				Expect(err).NotTo(HaveOccurred())
				Expect(entry.String()).To(Equal(fmt.Sprintf("%d", i)))
				Expect(entry.GetWorkload()).To(Equal("test"))
			}
			// then

			entry, err := fifolog.ReadLine()
			Expect(entry).To(BeNil())
			Expect(err).To(BeNil())
		})
	})
})
