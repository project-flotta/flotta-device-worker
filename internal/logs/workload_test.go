package logs_test

import (
	"errors"
	"net"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/logs"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-device-worker/internal/workload/api"
	"github.com/project-flotta/flotta-operator/models"
)

var _ = Describe("WorkloadsLogsTarget", func() {

	var (
		datadir   string
		mockCtrl  *gomock.Controller
		wkManager *workload.WorkloadManager
		wkwMock   *workload.MockWorkloadWrapper
		err       error
		l         net.Listener
	)

	const (
		deviceId = "foo"
	)

	BeforeEach(func() {
		l, err = net.Listen("tcp", "127.0.0.1:10600")
		Expect(err).ToNot(HaveOccurred())

		datadir, err = os.MkdirTemp("", "worloadTest")
		Expect(err).ToNot(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())
		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)

		wkwMock.EXPECT().Init().Return(nil).AnyTimes()
		wkManager, err = workload.NewWorkloadManagerWithParamsAndInterval(datadir, wkwMock, 2, deviceId)
		Expect(err).NotTo(HaveOccurred(), "cannot start the Workload Manager")

	})

	AfterEach(func() {
		l.Close()
		mockCtrl.Finish()
		_ = os.RemoveAll(datadir)
	})

	Context("Update", func() {
		var (
			work *logs.WorkloadsLogsTarget
		)

		BeforeEach(func() {
			work = logs.NewWorkloadsLogsTarget(wkManager)
		})

		It("Create all targets", func() {
			// given
			config := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					LogCollection: map[string]models.LogsCollectionInformation{
						"syslog": {
							BufferSize: 10,
							Kind:       "syslog",
							SyslogConfig: &models.LogsCollectionInformationSyslogConfig{
								Address:  "127.0.0.1:10600",
								Protocol: "tcp",
							},
						},
						"foo": {
							BufferSize: 10,
							Kind:       "syslog",
							SyslogConfig: &models.LogsCollectionInformationSyslogConfig{
								Address:  "127.0.0.1:10600",
								Protocol: "tcp",
							},
						},
					},
				},
				DeviceID:  deviceId,
				Workloads: []*models.Workload{},
			}
			// when
			err := work.Update(config)
			// then
			Expect(err).NotTo(HaveOccurred())
			transports := work.GetCurrentTransports()
			Expect(transports).To(HaveLen(2))
			Expect(transports).To(ContainElements("syslog", "foo"))

		})

		It("Leftover are deleted", func() {
			// given
			config := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					LogCollection: map[string]models.LogsCollectionInformation{
						"syslog": {
							BufferSize: 10,
							Kind:       "syslog",
							SyslogConfig: &models.LogsCollectionInformationSyslogConfig{
								Address:  "127.0.0.1:10600",
								Protocol: "tcp",
							},
						},
						"foo": {
							BufferSize: 10,
							Kind:       "syslog",
							SyslogConfig: &models.LogsCollectionInformationSyslogConfig{
								Address:  "127.0.0.1:10600",
								Protocol: "tcp",
							},
						},
					},
				},
				DeviceID: deviceId,
				Workloads: []*models.Workload{
					{Name: "work1", LogCollection: "syslog"},
					{Name: "work2", LogCollection: "foo"},
				},
			}

			err := work.Update(config)
			Expect(err).NotTo(HaveOccurred())
			transports := work.GetCurrentTransports()
			Expect(transports).To(HaveLen(2))
			Expect(transports).To(ContainElements("syslog", "foo"))

			// to check that workload is also removed if the target is remved
			wkwMock.EXPECT().Logs("work1", gomock.Any()).Times(1)
			wkwMock.EXPECT().Logs("work2", gomock.Any()).Times(1)
			work.WorkloadStarted("work1", nil)
			work.WorkloadStarted("work2", nil)
			res := work.GetCurrentWorkloads()
			Expect(res).To(HaveLen(2))

			// when

			config = models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					LogCollection: map[string]models.LogsCollectionInformation{
						"foo": {
							BufferSize: 10,
							Kind:       "syslog",
							SyslogConfig: &models.LogsCollectionInformationSyslogConfig{
								Address:  "127.0.0.1:10600",
								Protocol: "tcp",
							},
						},
					},
				},
				DeviceID:  deviceId,
				Workloads: []*models.Workload{},
			}
			err = work.Update(config)

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(work.GetCurrentTransports()).To(Equal([]string{"foo"}))

			res = work.GetCurrentWorkloads()
			Expect(res).To(HaveLen(1))
		})

		It("Invalid targets are not created", func() {
			// given
			config := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					LogCollection: map[string]models.LogsCollectionInformation{
						"syslog": {
							BufferSize: 10,
							Kind:       "INVALID",
							SyslogConfig: &models.LogsCollectionInformationSyslogConfig{
								Address:  "127.0.0.1:10600",
								Protocol: "tcp",
							},
						},
					},
				},
				DeviceID:  deviceId,
				Workloads: []*models.Workload{},
			}
			// when
			err := work.Update(config)
			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(work.GetCurrentTransports()).NotTo(Equal([]string{"foo"}))
		})
	})

	Context("Workload is modified", func() {
		var (
			work   *logs.WorkloadsLogsTarget
			config models.DeviceConfigurationMessage
		)

		BeforeEach(func() {
			work = logs.NewWorkloadsLogsTarget(wkManager)

			config = models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					LogCollection: map[string]models.LogsCollectionInformation{
						"syslog": {
							BufferSize: 10,
							Kind:       "syslog",
							SyslogConfig: &models.LogsCollectionInformationSyslogConfig{
								Address:  "127.0.0.1:10600",
								Protocol: "tcp",
							},
						},
					},
				},
				DeviceID:  deviceId,
				Workloads: []*models.Workload{},
			}
		})

		It("WorkloadStarted not present", func() {
			// given
			err := work.Update(config)
			Expect(err).NotTo(HaveOccurred())

			// when
			work.WorkloadStarted("InvalidWorkload", nil)
			res := work.GetCurrentWorkloads()
			// then
			Expect(res).To(HaveLen(0))
		})

		It("Workload with invalid target", func() {
			// given
			config.Workloads = []*models.Workload{
				{Name: "foo", LogCollection: "test"},
			}
			err := work.Update(config)

			Expect(err).NotTo(HaveOccurred())

			// when
			work.WorkloadStarted("foo", nil)
			res := work.GetCurrentWorkloads()
			// then
			Expect(res).To(HaveLen(0))
		})

		It("Workload with valid target", func() {
			// given
			config.Workloads = []*models.Workload{
				{Name: "foo", LogCollection: "syslog"},
			}
			err := work.Update(config)
			Expect(err).NotTo(HaveOccurred())

			wkwMock.EXPECT().Logs("foo", gomock.Any()).Times(1)

			// when
			work.WorkloadStarted("foo", nil)
			res := work.GetCurrentWorkloads()

			// then
			Expect(res).To(HaveLen(1))

			// Removed correctly
			work.WorkloadRemoved("foo")
			res = work.GetCurrentWorkloads()
			Expect(res).To(HaveLen(0))
		})
	})

	Context("Init", func() {
		var (
			work   *logs.WorkloadsLogsTarget
			config models.DeviceConfigurationMessage
		)

		BeforeEach(func() {
			work = logs.NewWorkloadsLogsTarget(wkManager)

			config = models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{
					LogCollection: map[string]models.LogsCollectionInformation{
						"syslog": {
							BufferSize: 10,
							Kind:       "syslog",
							SyslogConfig: &models.LogsCollectionInformationSyslogConfig{
								Address:  "127.0.0.1:10600",
								Protocol: "tcp",
							},
						},
					},
				},
				DeviceID: deviceId,
				Workloads: []*models.Workload{
					{Name: "foo", LogCollection: "syslog"},
				}}
		})

		It("Cannot retrieve workloads list", func() {
			// given
			wkwMock.EXPECT().List().Times(1).Return(nil, errors.New("test"))
			// when
			err := work.Init(config)
			// then
			Expect(err).To(HaveOccurred())
		})

		It("Started workloads logs", func() {
			// given
			wkwMock.EXPECT().List().Times(1).Return([]api.WorkloadInfo{{Name: "foo"}}, nil)
			wkwMock.EXPECT().Logs("foo", gomock.Any()).Times(1)
			// when
			err := work.Init(config)
			// then
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
