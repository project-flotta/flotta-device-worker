package metrics_test

import (
	"time"

	gomock "github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/metrics"
	"github.com/project-flotta/flotta-device-worker/internal/workload/podman"
	"github.com/project-flotta/flotta-operator/models"
)

var _ = Describe("Workload", func() {

	var (
		mockCtrl   *gomock.Controller
		daemonMock *metrics.MockMetricsDaemon
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		daemonMock = metrics.NewMockMetricsDaemon(mockCtrl)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("WorkloadStarted", func() {
		var (
			report *podman.PodReport
		)

		BeforeEach(func() {
			report = &podman.PodReport{
				Id:   "id1",
				Name: "wrk1",
				Containers: []*podman.ContainerReport{{
					IPAddress: "192.168.1.1",
					Id:        "c1",
					Name:      "c1",
				}},
			}
		})

		It("workload is not present", func() {

			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)

			// when
			wrk.WorkloadStarted("wrk1", []*podman.PodReport{report})

			// then
			// nothing, no call to AddTarget
		})

		It("workload has no metrics config", func() {
			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)

			workloads := []*models.Workload{{
				Data:            &models.DataConfiguration{},
				ImageRegistries: &models.ImageRegistries{},
				Metrics:         nil,
				Name:            "wrk1",
				Specification:   "{}",
			},
			}

			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}
			err := wrk.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// when
			wrk.WorkloadStarted("wrk1", []*podman.PodReport{report})

			// then
			// nothing, no call to AddTarget
		})

		It("workload has metrics defined", func() {
			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)

			workloads := []*models.Workload{{
				Data:            &models.DataConfiguration{},
				ImageRegistries: &models.ImageRegistries{},
				Metrics: &models.Metrics{
					Containers: map[string]models.ContainerMetrics{},
					Interval:   50,
					Path:       "/custom_metrics",
					Port:       9000,
				},
				Name:          "wrk1",
				Specification: "{}",
			},
			}

			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}

			err := wrk.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// then
			daemonMock.EXPECT().AddTarget(
				"wrk1",
				[]string{"http://192.168.1.1:9000/custom_metrics"},
				50*time.Second,
				&metrics.PermissiveAllowList{}).Times(1)

			// when
			wrk.WorkloadStarted("wrk1", []*podman.PodReport{report})
		})

		It("workload has metrics and custom container config", func() {
			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)

			workloads := []*models.Workload{{
				Data:            &models.DataConfiguration{},
				ImageRegistries: &models.ImageRegistries{},
				Metrics: &models.Metrics{
					Containers: map[string]models.ContainerMetrics{
						"c1": {Port: 8888, Path: "/c1/"},
					},
					Interval: 50,
					Path:     "/custom_metrics",
					Port:     9000,
				},
				Name:          "wrk1",
				Specification: "{}",
			},
			}

			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}

			err := wrk.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// then
			daemonMock.EXPECT().AddTarget(
				"wrk1",
				[]string{"http://192.168.1.1:8888/c1/"},
				50*time.Second,
				&metrics.PermissiveAllowList{}).Times(1)

			// when
			wrk.WorkloadStarted("wrk1", []*podman.PodReport{report})
		})

		It("Empty paths are correct", func() {
			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)

			workloads := []*models.Workload{{
				Data:            &models.DataConfiguration{},
				ImageRegistries: &models.ImageRegistries{},
				Metrics: &models.Metrics{
					Containers: map[string]models.ContainerMetrics{
						"c2": {Port: 8888, Path: ""},
					},
					Interval: 50,
					Path:     "",
					Port:     9000,
				},
				Name:          "wrk1",
				Specification: "{}",
			},
			}

			report = &podman.PodReport{
				Id:   "id1",
				Name: "wrk1",
				Containers: []*podman.ContainerReport{
					{IPAddress: "192.168.1.1", Id: "c1", Name: "c1"},
					{IPAddress: "192.168.1.2", Id: "c2", Name: "c2"},
				},
			}

			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}

			err := wrk.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// then
			daemonMock.EXPECT().AddTarget(
				"wrk1",
				[]string{"http://192.168.1.1:9000/", "http://192.168.1.2:8888/"},
				50*time.Second,
				&metrics.PermissiveAllowList{}).Times(1)

			// when
			wrk.WorkloadStarted("wrk1", []*podman.PodReport{report})
		})

		It("Disabled containers are not added", func() {
			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)

			workloads := []*models.Workload{{
				Data:            &models.DataConfiguration{},
				ImageRegistries: &models.ImageRegistries{},
				Metrics: &models.Metrics{
					Containers: map[string]models.ContainerMetrics{
						"c1": {Disabled: true, Port: 8888, Path: "/c1/"},
						"c2": {Disabled: false, Port: 8888, Path: "/c2/"},
					},
					Interval: 50,
					Path:     "/custom_metrics",
					Port:     9000,
				},
				Name:          "wrk1",
				Specification: "{}",
			},
			}

			report = &podman.PodReport{
				Id:   "id1",
				Name: "wrk1",
				Containers: []*podman.ContainerReport{
					{IPAddress: "192.168.1.1", Id: "c1", Name: "c1"},
					{IPAddress: "192.168.1.2", Id: "c2", Name: "c2"},
				},
			}

			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}
			err := wrk.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// then
			daemonMock.EXPECT().AddTarget(
				"wrk1",
				[]string{"http://192.168.1.2:8888/c2/"},
				50*time.Second,
				&metrics.PermissiveAllowList{}).Times(1)

			// when
			wrk.WorkloadStarted("wrk1", []*podman.PodReport{report})
		})

		It("Containers config does not get overwritten", func() {
			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)

			workloads := []*models.Workload{{
				Data:            &models.DataConfiguration{},
				ImageRegistries: &models.ImageRegistries{},
				Metrics: &models.Metrics{
					Containers: map[string]models.ContainerMetrics{
						"c1": {Port: 8888, Path: "/c1/"},
						"c3": {Port: 8888, Path: "/c3/"},
					},
					Interval: 50,
					Path:     "/custom_metrics",
					Port:     9000,
				},
				Name:          "wrk1",
				Specification: "{}",
			},
			}
			report = &podman.PodReport{
				Id:   "id1",
				Name: "wrk1",
				Containers: []*podman.ContainerReport{
					{IPAddress: "192.168.1.1", Id: "c1", Name: "c1"},
					{IPAddress: "192.168.1.2", Id: "c2", Name: "c2"},
					{IPAddress: "192.168.1.3", Id: "c3", Name: "c3"},
				},
			}
			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}

			err := wrk.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// then
			daemonMock.EXPECT().AddTarget(
				"wrk1",
				[]string{"http://192.168.1.1:8888/c1/", "http://192.168.1.2:9000/custom_metrics", "http://192.168.1.3:8888/c3/"},
				50*time.Second,
				&metrics.PermissiveAllowList{}).Times(1)

			// when
			wrk.WorkloadStarted("wrk1", []*podman.PodReport{report})
		})

		It("workload has metrics AllowList defined", func() {
			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)
			allowList := &models.MetricsAllowList{Names: []string{"test"}}
			workloads := []*models.Workload{{
				Data:            &models.DataConfiguration{},
				ImageRegistries: &models.ImageRegistries{},
				Metrics: &models.Metrics{
					Containers: map[string]models.ContainerMetrics{},
					Interval:   50,
					Path:       "/custom_metrics",
					Port:       9000,
					AllowList:  allowList,
				},
				Name:          "wrk1",
				Specification: "{}",
			},
			}

			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}

			err := wrk.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// then
			daemonMock.EXPECT().AddTarget(
				"wrk1",
				[]string{"http://192.168.1.1:9000/custom_metrics"},
				50*time.Second,
				metrics.NewRestrictiveAllowList(allowList)).Times(1)

			// when
			wrk.WorkloadStarted("wrk1", []*podman.PodReport{report})
		})

		It("workload has metrics AllowList empty", func() {
			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)
			workloads := []*models.Workload{{
				Data:            &models.DataConfiguration{},
				ImageRegistries: &models.ImageRegistries{},
				Metrics: &models.Metrics{
					Containers: map[string]models.ContainerMetrics{},
					Interval:   50,
					Path:       "/custom_metrics",
					Port:       9000,
					AllowList:  &models.MetricsAllowList{Names: []string{}},
				},
				Name:          "wrk1",
				Specification: "{}",
			},
			}

			cfg := models.DeviceConfigurationMessage{
				Workloads: workloads,
			}
			err := wrk.Update(cfg)
			Expect(err).NotTo(HaveOccurred())

			// then
			daemonMock.EXPECT().AddTarget(
				"wrk1",
				[]string{"http://192.168.1.1:9000/custom_metrics"},
				50*time.Second,
				metrics.NewRestrictiveAllowList(nil)).Times(1)

			// when
			wrk.WorkloadStarted("wrk1", []*podman.PodReport{report})
		})

	})

	Context("WorkloadStoped", func() {

		It("Removed correctly", func() {

			// given
			wrk := metrics.NewWorkloadMetrics(daemonMock)

			// then
			daemonMock.EXPECT().DeleteTarget("wrk1").Times(1)

			// when
			wrk.WorkloadRemoved("wrk1")
		})

	})

})
