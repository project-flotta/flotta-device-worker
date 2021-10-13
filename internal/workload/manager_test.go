package workload_test

import (
	"fmt"
	"io/ioutil"
	"os"

	gomock "github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	"github.com/jakub-dzon/k4e-device-worker/internal/workload"
	api "github.com/jakub-dzon/k4e-device-worker/internal/workload/api"
	"github.com/jakub-dzon/k4e-operator/models"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {

	var (
		datadir   string
		mockCtrl  *gomock.Controller
		wkManager *workload.WorkloadManager
		wkwMock   *workload.MockWorkloadWrapper
		err       error
	)

	BeforeEach(func() {
		datadir, err = ioutil.TempDir("", "worloadTest")
		Expect(err).ToNot(HaveOccurred())

		mockCtrl = gomock.NewController(GinkgoT())
		wkwMock = workload.NewMockWorkloadWrapper(mockCtrl)

		wkwMock.EXPECT().Init().Return(nil).AnyTimes()
		wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock)
		Expect(err).NotTo(HaveOccurred(), "Cannot start the Workload Manager")

	})

	AfterEach(func() {
		mockCtrl.Finish()
		_ = os.Remove(datadir)
	})

	Context("NewWorkloadManagerWithParams", func() {
		// @INFO: Other rules/creation correctly is part of the BeforeEach

		It("Testing invalid datadir", func() {

			// given
			datadir, err = ioutil.TempDir("", "worloadTest")
			err = os.Chmod(datadir, 0444)
			Expect(err).NotTo(HaveOccurred())

			// When
			wkManager, err = workload.NewWorkloadManagerWithParams(datadir, wkwMock)

			// Then
			Expect(err).To(HaveOccurred())
			Expect(wkManager).To(BeNil())
		})
	})

	Context("Update", func() {
		It("works as expected", func() {

			// given
			workloads := []*models.Workload{}

			for i := 0; i < 10; i++ {
				wkName := fmt.Sprintf("test%d", i)
				workloads = append(workloads, &models.Workload{
					Data:          &models.DataConfiguration{},
					Name:          wkName,
					Specification: "{}",
				})
				wkwMock.EXPECT().Remove(wkName).Times(1)
			}

			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads:     workloads,
			}

			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any()).AnyTimes()

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
		})

		It("Workload Run failed", func() {
			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads: []*models.Workload{{
					Data:          &models.DataConfiguration{},
					Name:          "test",
					Specification: "{}",
				}},
			}

			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().Remove("test").AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any()).Return(fmt.Errorf("Cannot run workload")).Times(1)

			// when
			err := wkManager.Update(cfg)
			merr, _ := err.(*multierror.Error)

			// then
			Expect(err).To(HaveOccurred())
			Expect(merr.WrappedErrors()).To(HaveLen(1))
		})

		It("Cannot remove existing workload", func() {
			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads: []*models.Workload{{
					Data:          &models.DataConfiguration{},
					Name:          "test",
					Specification: "{}",
				}},
			}

			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().Remove("test").Return(fmt.Errorf("Cannot run workload")).Times(1)
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any()).Times(0)

			err := wkManager.Update(cfg)
			merr, _ := err.(*multierror.Error)

			// then
			Expect(err).To(HaveOccurred())
			Expect(merr.WrappedErrors()).To(HaveLen(1))
		})

		It("Some workloads failed", func() {
			// So make sure that all worksloads tried to be executed, even if one
			// failed.

			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads: []*models.Workload{
					{
						Data:          &models.DataConfiguration{},
						Name:          "test",
						Specification: "{}",
					},
					{
						Data:          &models.DataConfiguration{},
						Name:          "testB",
						Specification: "{}",
					},
				},
			}

			wkwMock.EXPECT().List().AnyTimes()
			wkwMock.EXPECT().Remove("test").AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), getManifest(datadir, "test")).Return(fmt.Errorf("Cannot run workload")).Times(1)

			wkwMock.EXPECT().Remove("testB").AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), getManifest(datadir, "testB")).Return(nil).Times(1)

			// when
			err := wkManager.Update(cfg)
			merr, _ := err.(*multierror.Error)

			// then
			Expect(err).To(HaveOccurred())
			Expect(merr.WrappedErrors()).To(HaveLen(1))
		})

		It("staled workload got deleted if it's not in the config", func() {
			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads: []*models.Workload{
					{
						Data:          &models.DataConfiguration{},
						Name:          "test",
						Specification: "{}",
					},
					{
						Data:          &models.DataConfiguration{},
						Name:          "testB",
						Specification: "{}",
					},
				},
			}

			currentWorkloads := []api.WorkloadInfo{
				{Id: "stale", Name: "stale", Status: "running"},
			}
			wkwMock.EXPECT().List().Return(currentWorkloads, nil).AnyTimes()

			wkwMock.EXPECT().Remove("test").AnyTimes()
			wkwMock.EXPECT().Remove("testB").AnyTimes()
			wkwMock.EXPECT().Run(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			wkwMock.EXPECT().Remove("stale").Times(1)

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).NotTo(HaveOccurred())
		})

		It("staled workload cannot get deleted", func() {
			// given
			cfg := models.DeviceConfigurationMessage{
				Configuration: &models.DeviceConfiguration{Heartbeat: &models.HeartbeatConfiguration{PeriodSeconds: 1}},
				DeviceID:      "",
				Version:       "",
				Workloads:     []*models.Workload{},
			}

			currentWorkloads := []api.WorkloadInfo{
				{Id: "stale", Name: "stale", Status: "running"},
			}
			wkwMock.EXPECT().List().Return(currentWorkloads, nil).AnyTimes()

			wkwMock.EXPECT().Remove("stale").Return(fmt.Errorf("invalid workload"))

			// when
			err := wkManager.Update(cfg)

			// then
			Expect(err).To(HaveOccurred())
		})
	})

	Context("ListWorkloads", func() {
		It("Return the list correctly", func() {

			// given
			currentWorkloads := []api.WorkloadInfo{
				{Id: "foo", Name: "foo", Status: "running"},
			}
			wkwMock.EXPECT().List().Return(currentWorkloads, nil).AnyTimes()

			// when

			list, err := wkManager.ListWorkloads()

			// then

			Expect(list).To(Equal(currentWorkloads))
			Expect(err).NotTo(HaveOccurred())
		})

		It("Return error correctly", func() {

			// given
			currentWorkloads := []api.WorkloadInfo{}
			wkwMock.EXPECT().List().Return(currentWorkloads, fmt.Errorf("Invalid")).AnyTimes()

			// when

			list, err := wkManager.ListWorkloads()

			// then

			Expect(list).To(Equal(currentWorkloads))
			Expect(err).To(HaveOccurred())
		})

	})
})

func getManifest(datadir string, workloadName string) string {
	return fmt.Sprintf("%s/manifests/%s.yaml", datadir, workloadName)
}
