package workload_test

import (
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/network"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/podman"
	"github.com/jakub-dzon/k4e-operator/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman services", func() {

	var (
		datadir  string
		mockCtrl *gomock.Controller
		wk       *workload.Workload
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		newPodman := podman.NewMockPodman(mockCtrl)
		netfilter := network.NewMockNetfilter(mockCtrl)

		wk = workload.Workload{
			workloads:          newPodman,
			netfilter:          netfilter,
			mappingRepository:  mappingRepository,
			serviceManager:     serviceManager,
			monitoringInterval: defaultWorkloadsMonitoringInterval,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("NonDefaultMonitoringInterval", func() {

		It("Emit events in case of Start failure", func() {

			// given
			workloads := []*models.Workload{}
			workloads = append(workloads, &models.Workload{
				Data:          &models.DataConfiguration{},
				Name:          "stale",
				Specification: "{}",
			})
			wkwMock.EXPECT().Remove("stale").Times(1)

			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads:     workloads,
			}

			wkwMock.EXPECT().ListSecrets().Return(nil, nil).AnyTimes()
			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{
				{Id: "stale", Name: "stale", Status: "created"},
			}, nil).AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("Failed to start container"))
			wkwMock.EXPECT().PersistConfiguration().AnyTimes()
			wkwMock.EXPECT().Start(gomock.Any()).Return(fmt.Errorf("failed to start container")).AnyTimes()

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())

			// Check no events are generated:
			time.Sleep(5 * time.Second)
			events := wkManager.PopEvents()
			Expect(len(events)).To(BeNumerically(">=", 1))
		})
	})

})
