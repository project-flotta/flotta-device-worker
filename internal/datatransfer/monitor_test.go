package datatransfer_test

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/project-flotta/flotta-device-worker/internal/configuration"
	"github.com/project-flotta/flotta-device-worker/internal/datatransfer"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-device-worker/internal/workload/api"
	"github.com/project-flotta/flotta-operator/models"
)

var _ = Describe("Datatransfer", func() {

	var (
		datadir       string
		mockCtrl      *gomock.Controller
		wkManager     *workload.WorkloadManager
		configManager *configuration.Manager
		wkwMock       *workload.MockWorkloadWrapper
		err           error
		cfg           models.DeviceConfigurationMessage
	)

	BeforeEach(func() {

		datadir, err = ioutil.TempDir("", "worloadTest")
		Expect(err).ToNot(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())
		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)

		wkwMock.EXPECT().Init().Return(nil).AnyTimes()

		wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock, "device-id-123")
		Expect(err).NotTo(HaveOccurred(), "Cannot start the Workload Manager")

		configManager = configuration.NewConfigurationManager(datadir)
		fmt.Println(configManager)

		deviceConfiguration := models.DeviceConfiguration{
			Heartbeat: &models.HeartbeatConfiguration{
				PeriodSeconds: 1,
			},
			Storage: &models.StorageConfiguration{
				S3: &models.S3StorageConfiguration{
					AwsAccessKeyID:     "",
					AwsCaBundle:        "",
					AwsSecretAccessKey: "",
					BucketHost:         "",
					BucketName:         "",
					BucketPort:         0,
				},
			},
		}

		cfg = models.DeviceConfigurationMessage{
			Configuration: &deviceConfiguration,
			DeviceID:      "",
			Version:       "",
			Workloads: []*models.Workload{
				{
					Data: &models.DataConfiguration{
						Egress: []*models.DataPath{{
							Source: "/metrics",
							Target: "/metrics",
						}},
						Ingress: []*models.DataPath{{
							Source: "/remote/data",
							Target: "/data",
						}},
					},
					Name:          "test",
					Specification: "{}",
				},

				{
					Data: &models.DataConfiguration{
						Egress: []*models.DataPath{{
							Source: "/metrics",
							Target: "/metrics",
						}},
					},
					Name:          "foo",
					Specification: "{}",
				},
				{
					Data: &models.DataConfiguration{
						Egress: []*models.DataPath{{
							Source: "/metrics",
							Target: "/metrics",
						}},
						Ingress: []*models.DataPath{{
							Source: "/remote/configuration",
							Target: "/workload/configuration",
						}},
					},
					Name:          "bar",
					Specification: "{}",
				},
			},
			WorkloadsMonitoringInterval: 0,
		}

		err := configManager.Update(cfg)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		mockCtrl.Finish()
		_ = os.Remove(datadir)
	})

	Context("Sync", func() {

		It("Work as expected", func() {

			// given
			fssync := datatransfer.NewMockFileSync(mockCtrl)
			fssync.EXPECT().Connect().Times(2)
			fssync.EXPECT().Disconnect().Times(2)
			fssync.EXPECT().SyncPath(gomock.Any(), gomock.Any()).AnyTimes()
			fssync.EXPECT().GetStatistics().Times(2)

			monitor := datatransfer.NewMonitor(wkManager, configManager)
			monitor.SetStorage(fssync)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{{
				Id:     "test",
				Name:   "test",
				Status: "Running",
			}}, nil).AnyTimes()

			// when
			err := monitor.ForceSync()

			// then
			Expect(err).NotTo(HaveOccurred())
		})

		It("One sync failed", func() {
			// given
			fssync := datatransfer.NewMockFileSync(mockCtrl)
			fssync.EXPECT().Connect().Times(5)
			fssync.EXPECT().Disconnect().Times(5)
			fssync.EXPECT().SyncPath(gomock.Any(), gomock.Any()).Times(2)
			fssync.EXPECT().SyncPath(gomock.Any(), gomock.Any()).Return(fmt.Errorf("failed")).Times(1)
			fssync.EXPECT().SyncPath(gomock.Any(), gomock.Any()).Times(2)
			fssync.EXPECT().GetStatistics().Times(5)

			monitor := datatransfer.NewMonitor(wkManager, configManager)
			monitor.SetStorage(fssync)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{
				{Id: "test", Name: "test", Status: "Running"},
				{Id: "foo", Name: "foo", Status: "Running"},
				{Id: "bar", Name: "bar", Status: "Running"},
			}, nil).AnyTimes()

			// when
			err := monitor.ForceSync()

			// then
			Expect(err).To(HaveOccurred())
			Expect(monitor.GetLastSuccessfulSyncTime("test")).NotTo(BeNil())
			Expect(monitor.GetLastSuccessfulSyncTime("foo")).To(BeNil())
			Expect(monitor.GetLastSuccessfulSyncTime("bar")).NotTo(BeNil())
		})

		It("Cannot retrieve workloads", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)

			wkwMock.EXPECT().List().Return(nil, fmt.Errorf("failed")).Times(1)

			// when
			err := monitor.ForceSync()

			// then
			Expect(err).To(HaveOccurred())
		})

		It("Cannot connect to external storage", func() {
			// given
			fssync := datatransfer.NewMockFileSync(mockCtrl)
			fssync.EXPECT().Connect().Return(fmt.Errorf("failed")).Times(3)

			monitor := datatransfer.NewMonitor(wkManager, configManager)
			monitor.SetStorage(fssync)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{
				{Id: "test", Name: "test", Status: "Running"},
				{Id: "foo", Name: "foo", Status: "Running"},
				{Id: "bar", Name: "bar", Status: "Running"},
			}, nil).AnyTimes()

			// when
			err := monitor.ForceSync()

			// then
			Expect(err).To(HaveOccurred())
		})

		It("Does not report workloads without path", func() {
			// given
			fssync := datatransfer.NewMockFileSync(mockCtrl)
			fssync.EXPECT().Connect().Times(2)
			fssync.EXPECT().Disconnect().Times(2)
			fssync.EXPECT().SyncPath(gomock.Any(), gomock.Any()).Times(2)
			fssync.EXPECT().GetStatistics().Times(2)

			monitor := datatransfer.NewMonitor(wkManager, configManager)
			monitor.SetStorage(fssync)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{
				{Id: "test", Name: "test", Status: "Running"},
				{Id: "invalid", Name: "invalid", Status: "Running"},
			}, nil).AnyTimes()

			// when
			err := monitor.ForceSync()

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(monitor.GetLastSuccessfulSyncTime("test")).NotTo(BeNil())
		})

		It("Does nothing if there are no workloads", func() {

			// given
			fssync := datatransfer.NewMockFileSync(mockCtrl)
			fssync.EXPECT().Connect().Times(0)
			fssync.EXPECT().SyncPath(gomock.Any(), gomock.Any()).Times(0)

			monitor := datatransfer.NewMonitor(wkManager, configManager)
			monitor.SetStorage(fssync)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{}, nil).AnyTimes()

			// when
			err := monitor.ForceSync()

			// then
			Expect(err).NotTo(HaveOccurred())
		})

	})

	Context("WorkloadRemoved", func() {
		It("Removed correctly", func() {
			// given
			fssync := datatransfer.NewMockFileSync(mockCtrl)
			fssync.EXPECT().Connect().Times(2)
			fssync.EXPECT().Disconnect().Times(2)
			fssync.EXPECT().SyncPath(gomock.Any(), gomock.Any()).Times(2)
			fssync.EXPECT().GetStatistics().Times(2)

			monitor := datatransfer.NewMonitor(wkManager, configManager)
			monitor.SetStorage(fssync)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{
				{Id: "test", Name: "test", Status: "Running"},
			}, nil).AnyTimes()

			// when
			monitor.WorkloadRemoved("test")

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(monitor.GetLastSuccessfulSyncTime("test")).To(BeNil())
		})
	})

	Context("HasStorage", func() {
		It("True if it's defined", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)

			// when
			res := monitor.HasStorageDefined()

			// then
			Expect(res).To(BeTrue())
		})

		It("False no storage defined", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)
			cfg.Configuration.Storage = nil
			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			res := monitor.HasStorageDefined()

			// then
			Expect(res).To(BeFalse())
		})

		It("False no s3-storage defined", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)
			cfg.Configuration.Storage.S3 = nil
			err := configManager.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			res := monitor.HasStorageDefined()

			// then
			Expect(res).To(BeFalse())
		})

	})

	Context("Init", func() {
		It("Works with valid configuration", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)

			// when
			err := monitor.Init(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
		})

		It("Ignored missing storage configuration on init", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)
			cfg.Configuration = nil

			// when
			err := monitor.Init(cfg)

			// then
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("Update", func() {
		It("Works with valid configuration", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)

			// when
			err := monitor.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
		})

		It("Ignored invalid device configuration on update", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)
			cfg.Configuration = nil

			// when
			err := monitor.Update(cfg)

			// then
			Expect(err).ToNot(HaveOccurred())
		})

		It("Ignored missing storage configuration", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)
			cfg.Configuration.Storage = nil

			// when
			err := monitor.Update(cfg)

			// then
			Expect(err).ToNot(HaveOccurred())
		})

		It("Failed with invalid s3 configuration", func() {
			// given
			monitor := datatransfer.NewMonitor(wkManager, configManager)
			cfg.Configuration.Storage.S3.AwsAccessKeyID = "invalidbase64"

			// when
			err := monitor.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())
		})
	})

})
