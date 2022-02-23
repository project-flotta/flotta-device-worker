package logs_test

import (
	"errors"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/logs"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
)

var _ = Describe("WorkloadWrite", func() {

	var (
		datadir   string
		mockCtrl  *gomock.Controller
		wkManager *workload.WorkloadManager
		wkwMock   *workload.MockWorkloadWrapper
		err       error
		target    *logs.FIFOLog
	)

	const (
		deviceId = "foo"
	)

	BeforeEach(func() {
		datadir, err = ioutil.TempDir("", "worloadTest")
		Expect(err).ToNot(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())
		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)

		wkwMock.EXPECT().Init().Return(nil).AnyTimes()
		wkManager, err = workload.NewWorkloadManagerWithParamsAndInterval(datadir, wkwMock, 2, deviceId)
		Expect(err).NotTo(HaveOccurred(), "cannot start the Workload Manager")
		target = logs.NewFIFOLog(5)
	})

	AfterEach(func() {
		mockCtrl.Finish()
		_ = os.RemoveAll(datadir)
	})

	Context("Write", func() {

		It("No target defined", func() {
			// given
			writer := logs.NewWorkloadWriter("workload", wkManager)
			// when
			n, err := writer.Write([]byte("foo"))
			// then
			Expect(err).To(HaveOccurred())
			Expect(n).To(Equal(0))
		})

		It("Target is defined correctly", func() {

			// given
			writer := logs.NewWorkloadWriter("workload", wkManager)
			writer.SetTarget("test", target)

			// when
			n, err := writer.Write([]byte("foo"))

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(3))
		})

		It("Target failed to write", func() {

			// given
			writer := logs.NewWorkloadWriter("workload", wkManager)
			writer.SetTarget("test", target)

			n, err := writer.Write([]byte("123456789"))
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(9))

			// when
			n, err = writer.Write([]byte("123456789"))

			// then
			Expect(err).To(HaveOccurred())
			Expect(n).To(Equal(0))
		})
	})

	Context("Start/stop operations", func() {
		It("Cannot retrieve logs", func() {
			// given
			writer := logs.NewWorkloadWriter("workload", wkManager)
			writer.SetTarget("test", target)
			// then

			wkwMock.EXPECT().Logs("workload", gomock.Any()).Times(1).Return(nil, errors.New("failed"))

			// when
			err := writer.Start()
			Expect(err).To(HaveOccurred())

			// just to make sure that it's working as expected
			writer.Stop()
		})

		It("Retrieve logs correctly", func() {
			// given
			writer := logs.NewWorkloadWriter("workload", wkManager)
			writer.SetTarget("test", target)
			// then

			wkwMock.EXPECT().Logs("workload", gomock.Any()).Times(1)

			// when
			err := writer.Start()
			Expect(err).NotTo(HaveOccurred())

			// just to make sure that it's working as expected
			writer.Stop()
		})

	})
})
