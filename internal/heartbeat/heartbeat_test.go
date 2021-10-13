package heartbeat_test

import (
	"context"
	"fmt"

	"github.com/golang/mock/gomock"
	"github.com/jakub-dzon/k4e-device-worker/internal/configuration"
	"github.com/jakub-dzon/k4e-device-worker/internal/datatransfer"
	"github.com/jakub-dzon/k4e-device-worker/internal/hardware"
	"github.com/jakub-dzon/k4e-device-worker/internal/heartbeat"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-operator/models"
	"google.golang.org/grpc"

	pb "github.com/redhatinsights/yggdrasil/protocol"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Heartbeat", func() {

	var (
		datadir       = "/tmp"
		mockCtrl      *gomock.Controller
		wkManager     *workload.WorkloadManager
		configManager *configuration.Manager
		wkwMock       *workload.MockWorkloadWrapper
		hw            = &hardware.Hardware{}
		monitor       = &datatransfer.Monitor{}
		hb            = &heartbeat.Heartbeat{}
		err           error
		client        Dispatcher
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)

		wkwMock.EXPECT().Init().Return(nil).AnyTimes()
		wkwMock.EXPECT().PersistConfiguration().AnyTimes()

		wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock)
		Expect(err).NotTo(HaveOccurred(), "Cannot start the Workload Manager")

		configManager = configuration.NewConfigurationManager(datadir)
		client = Dispatcher{}
		hb = heartbeat.NewHeartbeatService(&client,
			configManager,
			wkManager,
			hw,
			monitor)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("HeartBeatData test", func() {
		It("Report empty workloads an up status", func() {
			//given
			wkwMock.EXPECT().List().Times(1)
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, hw, monitor)

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then
			Expect(heartbeatInfo.Status).To(Equal("up"))
			Expect(heartbeatInfo.Workloads).To(BeEmpty())
		})

		It("Report workload correctly", func() {
			//given
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, hw, monitor)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{{
				Id:     "test",
				Name:   "test",
				Status: "Running",
			}}, nil).AnyTimes()

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then
			Expect(heartbeatInfo.Status).To(Equal("up"))

			// Workload checks
			Expect(heartbeatInfo.Workloads).To(HaveLen(1))
			Expect(heartbeatInfo.Workloads[0].Name).To(Equal("test"))
			Expect(heartbeatInfo.Workloads[0].Status).To(Equal("Running"))
		})

		It("Cannot retrieve the list of workloads", func() {
			//given
			hbData := heartbeat.NewHeartbeatData(configManager, wkManager, hw, monitor)

			wkwMock.EXPECT().List().Return([]api.WorkloadInfo{}, fmt.Errorf("Invalid list")).AnyTimes()

			//when
			heartbeatInfo := hbData.RetrieveInfo()

			//then
			Expect(heartbeatInfo.Status).To(Equal("up"))
			Expect(heartbeatInfo.Workloads).To(HaveLen(0))
		})
	})

	Context("Start", func() {
		It("Ticker is stopped if it's not started", func() {
			//given
			Expect(hb.HasStarted()).To(BeFalse(), "Ticker is initialized when it shouldn't")

			// when
			hb.Start()

			//then
			Expect(hb.HasStarted()).To(BeTrue())
		})
	})

	Context("Update", func() {

		BeforeEach(func() {
			wkwMock.EXPECT().List().AnyTimes()
		})

		It("Ticker is created", func() {

			//given
			Expect(hb.HasStarted()).To(BeFalse(), "Ticker is initialized when it shouldn't")

			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads:     []*models.Workload{},
			}

			// when
			err := hb.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(hb.HasStarted()).To(BeTrue())
		})

		It("Ticker not created on invalid config", func() {

			// given
			Expect(hb.HasStarted()).To(BeFalse(), "Ticker is initialized when it shouldn't")

			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{},
				DeviceID:      "",
				Version:       "",
				Workloads:     []*models.Workload{},
			}

			// when
			err := hb.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
			Expect(hb.HasStarted()).To(BeTrue())
		})

	})

})

// We keep the latest send data to make sure that we validate the data sent to
// the operator without sent at all
type Dispatcher struct {
	latestData *pb.Data
}

func (d *Dispatcher) Send(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Receipt, error) {
	d.latestData = in
	return nil, nil
}

func (d *Dispatcher) Register(ctx context.Context, in *pb.RegistrationRequest, opts ...grpc.CallOption) (*pb.RegistrationResponse, error) {
	return nil, nil
}
