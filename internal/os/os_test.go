package os_test

import (
	"encoding/json"
	"fmt"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-operator/models"
	"testing"
	"time"

	os2 "github.com/project-flotta/flotta-device-worker/internal/os"
)

func TestOs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Os Suite")
}

const (
	RequestedCommitId         = "123"
	NewHostedUrl              = "789"
	UpgradeTime               = 1644322561
	UnknownBootedCommitID     = "1"
	UnknownUnBootableCommitID = "2"
	StatusFailed              = "failed"
	StatusSucceeded           = "succeeded"
	OldCommitID               = "oldCommitID"
	OldUpgradeTime            = "oldTimeStamp"
	OldUpgradeStatus          = StatusSucceeded
	UnBootableUpgradeTime     = 0
	AnotherCommitID           = "AnotherCommitID"
)

var _ = Describe("Os", func() {

	var (
		mockCtrl              *gomock.Controller
		gracefulRebootChannel chan struct{}
		deviceOS              *os2.OS
		osExecCommandsMock    *os2.MockOsExecCommands
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		osExecCommandsMock = os2.NewMockOsExecCommands(mockCtrl)
		gracefulRebootChannel = make(chan struct{})
		osExecCommandsMock.EXPECT().IsRpmOstreeAvailable().Return(true)
		deviceOS = os2.NewOS(gracefulRebootChannel, osExecCommandsMock)
		deviceOS.OsCommit = OldCommitID
		deviceOS.RequestedOsCommit = OldCommitID
		deviceOS.LastUpgradeTime = OldUpgradeTime
		deviceOS.LastUpgradeStatus = OldUpgradeStatus
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Context("Update OS status test", func() {

		It("Get upgrade status happy flow", func() {
			// given
			deviceOS.RequestedOsCommit = RequestedCommitId

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(returnMockRpmOstreeStatus(RequestedCommitId, UpgradeTime, OldCommitID))

			// when
			upgradeStatus := deviceOS.GetUpgradeStatus()

			//then
			Expect(upgradeStatus.CurrentCommitID).To(Equal(RequestedCommitId))
			Expect(upgradeStatus.LastUpgradeTime).To(Equal(time.Unix(int64(UpgradeTime), 0).String()))
			Expect(upgradeStatus.LastUpgradeStatus).To(Equal(StatusSucceeded))
		})

		It("Get upgrade status failed because of RpmOstreeStatus failure", func() {
			// given
			deviceOS.RequestedOsCommit = RequestedCommitId

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(nil, fmt.Errorf("Failed to get status"))

			// when
			upgradeStatus := deviceOS.GetUpgradeStatus()

			//then
			Expect(upgradeStatus.CurrentCommitID).To(Equal(OldCommitID))
			Expect(upgradeStatus.LastUpgradeTime).To(Equal(OldUpgradeTime))
			Expect(upgradeStatus.LastUpgradeStatus).To(Equal(OldUpgradeStatus))
		})

		It("Get upgrade status failed because of unmarshal command", func() {
			// given
			deviceOS.RequestedOsCommit = RequestedCommitId
			unmarshalJson, _ := json.Marshal("unmarshal")

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(unmarshalJson, nil)

			// when
			upgradeStatus := deviceOS.GetUpgradeStatus()

			//then
			Expect(upgradeStatus.CurrentCommitID).To(Equal(OldCommitID))
			Expect(upgradeStatus.LastUpgradeTime).To(Equal(OldUpgradeTime))
			Expect(upgradeStatus.LastUpgradeStatus).To(Equal(OldUpgradeStatus))
		})

		It("Get upgrade status failed because there isn't any deployment", func() {
			// given
			deviceOS.RequestedOsCommit = RequestedCommitId
			emptyDeployments, _ := json.Marshal(os2.StatusStruct{Deployments: []*os2.Deployments{}})
			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(emptyDeployments, nil)

			// when
			upgradeStatus := deviceOS.GetUpgradeStatus()

			//then
			Expect(upgradeStatus.CurrentCommitID).To(Equal(OldCommitID))
			Expect(upgradeStatus.LastUpgradeTime).To(Equal(OldUpgradeTime))
			Expect(upgradeStatus.LastUpgradeStatus).To(Equal(OldUpgradeStatus))
		})

		It("Get upgrade status failed because of unknown deployments", func() {
			// given
			deviceOS.RequestedOsCommit = RequestedCommitId

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(returnMockRpmOstreeStatus(UnknownBootedCommitID, UpgradeTime, UnknownUnBootableCommitID))

			// when
			upgradeStatus := deviceOS.GetUpgradeStatus()

			//then
			Expect(upgradeStatus.CurrentCommitID).To(Equal(UnknownBootedCommitID))
			Expect(upgradeStatus.LastUpgradeTime).To(Equal(OldUpgradeTime))
			Expect(upgradeStatus.LastUpgradeStatus).To(Equal(OldUpgradeStatus))
		})

		It("Get upgrade status failed because of upgrade failure", func() {
			// given
			deviceOS.RequestedOsCommit = RequestedCommitId

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(returnMockRpmOstreeStatus(OldCommitID, UpgradeTime, RequestedCommitId))

			// when
			upgradeStatus := deviceOS.GetUpgradeStatus()

			//then
			Expect(upgradeStatus.CurrentCommitID).To(Equal(OldCommitID))
			Expect(upgradeStatus.LastUpgradeTime).To(Equal(time.Unix(int64(UnBootableUpgradeTime), 0).String()))
			Expect(upgradeStatus.LastUpgradeStatus).To(Equal(StatusFailed))
		})

	})

	Context("Run Update OS test", func() {

		It("Changing the HostedURL", func() {
			// given
			cfg := createConfig(true, OldCommitID, NewHostedUrl)
			osExecCommandsMock.EXPECT().UpdateUrlInEdgeRemote(NewHostedUrl, os2.EdgeConfFileName).Return(nil)
			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(returnMockRpmOstreeBootedOnlyStatus(OldCommitID, UpgradeTime))

			// when
			err := deviceOS.Update(cfg)

			//then
			Expect(err).NotTo(HaveOccurred())
			upgradeStatusAfterUpdate := deviceOS.GetUpgradeStatus()
			Expect(upgradeStatusAfterUpdate.CurrentCommitID).To(Equal(OldCommitID))
			Expect(upgradeStatusAfterUpdate.LastUpgradeTime).To(Equal(time.Unix(int64(UpgradeTime), 0).String()))
			Expect(upgradeStatusAfterUpdate.LastUpgradeStatus).To(Equal(StatusSucceeded))
		})

		It("Changing the CommitId", func() {
			// given
			cfg := createConfig(true, RequestedCommitId, "")

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(returnMockRpmOstreeBootedOnlyStatus(RequestedCommitId, UpgradeTime))
			osExecCommandsMock.EXPECT().RpmOstreeUpdatePreview().Return(returnMockRpmUpdatePreview(RequestedCommitId))
			osExecCommandsMock.EXPECT().EnsureScriptExists(gomock.Any(), gomock.Any()).Return(nil).Times(2)
			osExecCommandsMock.EXPECT().RpmOstreeUpgrade().Return(nil)
			osExecCommandsMock.EXPECT().SystemReboot().Return(nil)
			waitForGracefulRebootChannel(deviceOS)

			// when
			err := deviceOS.Update(cfg)

			//then
			Expect(err).NotTo(HaveOccurred())
			upgradeStatusAfterUpdate := deviceOS.GetUpgradeStatus()
			Expect(upgradeStatusAfterUpdate.CurrentCommitID).To(Equal(RequestedCommitId))
			Expect(upgradeStatusAfterUpdate.LastUpgradeTime).To(Equal(time.Unix(int64(UpgradeTime), 0).String()))
			Expect(upgradeStatusAfterUpdate.LastUpgradeStatus).To(Equal(StatusSucceeded))
		})

		It("Changing the CommitId but automaticallyUpgrade is false so the upgrade wouldn't be triggered", func() {
			// given
			deviceOS.AutomaticallyUpgrade = false
			cfg := createConfig(false, RequestedCommitId, "")

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(returnMockRpmOstreeBootedOnlyStatus(OldCommitID, UpgradeTime))
			waitForGracefulRebootChannel(deviceOS)

			// when
			err := deviceOS.Update(cfg)

			//then
			Expect(err).NotTo(HaveOccurred())
			upgradeStatusAfterUpdate := deviceOS.GetUpgradeStatus()
			Expect(upgradeStatusAfterUpdate.CurrentCommitID).To(Equal(OldCommitID))
			Expect(upgradeStatusAfterUpdate.LastUpgradeTime).To(Equal(OldUpgradeTime))
			Expect(upgradeStatusAfterUpdate.LastUpgradeStatus).To(Equal(OldUpgradeStatus))
		})

		It("Changing the CommitId, but preview doesn't contain the requested comment so the upgrade wouldn't be triggered", func() {
			// given
			cfg := createConfig(true, RequestedCommitId, "")

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(returnMockRpmOstreeBootedOnlyStatus(OldCommitID, UpgradeTime))
			osExecCommandsMock.EXPECT().RpmOstreeUpdatePreview().Return(returnMockRpmUpdatePreview(AnotherCommitID))

			// when
			err := deviceOS.Update(cfg)

			//then
			Expect(err).To(HaveOccurred())
			upgradeStatusAfterUpdate := deviceOS.GetUpgradeStatus()
			Expect(upgradeStatusAfterUpdate.CurrentCommitID).To(Equal(OldCommitID))
			Expect(upgradeStatusAfterUpdate.LastUpgradeTime).To(Equal(OldUpgradeTime))
			Expect(upgradeStatusAfterUpdate.LastUpgradeStatus).To(Equal(OldUpgradeStatus))
		})

		It("Changing the CommitId, but rpmostree upgrade was failed", func() {
			// given
			cfg := createConfig(true, RequestedCommitId, "")

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(returnMockRpmOstreeBootedOnlyStatus(OldCommitID, UpgradeTime))
			osExecCommandsMock.EXPECT().RpmOstreeUpdatePreview().Return(returnMockRpmUpdatePreview(RequestedCommitId))
			osExecCommandsMock.EXPECT().EnsureScriptExists(gomock.Any(), gomock.Any()).Return(nil).Times(2)
			osExecCommandsMock.EXPECT().RpmOstreeUpgrade().Return(fmt.Errorf("Failed to upgrade"))

			// when
			err := deviceOS.Update(cfg)

			//then
			Expect(err).To(HaveOccurred())
			upgradeStatusAfterUpdate := deviceOS.GetUpgradeStatus()
			Expect(upgradeStatusAfterUpdate.CurrentCommitID).To(Equal(OldCommitID))
			Expect(upgradeStatusAfterUpdate.LastUpgradeTime).To(Equal(OldUpgradeTime))
			Expect(upgradeStatusAfterUpdate.LastUpgradeStatus).To(Equal(OldUpgradeStatus))
		})

		It("Changing the CommitId, but system reboot was failed", func() {
			// given
			cfg := createConfig(true, RequestedCommitId, "")

			osExecCommandsMock.EXPECT().RpmOstreeStatus().Return(returnMockRpmOstreeBootedOnlyStatus(OldCommitID, UpgradeTime))
			osExecCommandsMock.EXPECT().RpmOstreeUpdatePreview().Return(returnMockRpmUpdatePreview(RequestedCommitId))
			osExecCommandsMock.EXPECT().EnsureScriptExists(gomock.Any(), gomock.Any()).Return(nil).Times(2)
			osExecCommandsMock.EXPECT().RpmOstreeUpgrade().Return(nil)
			osExecCommandsMock.EXPECT().SystemReboot().Return(fmt.Errorf("Failed to reboot"))
			waitForGracefulRebootChannel(deviceOS)

			// when
			err := deviceOS.Update(cfg)

			//then
			Expect(err).To(HaveOccurred())
			upgradeStatusAfterUpdate := deviceOS.GetUpgradeStatus()
			Expect(upgradeStatusAfterUpdate.CurrentCommitID).To(Equal(OldCommitID))
			Expect(upgradeStatusAfterUpdate.LastUpgradeTime).To(Equal(OldUpgradeTime))
			Expect(upgradeStatusAfterUpdate.LastUpgradeStatus).To(Equal(OldUpgradeStatus))
		})
	})
})

var _ = Describe("OS management disabled", func() {

	var (
		mockCtrl              *gomock.Controller
		gracefulRebootChannel chan struct{}
		deviceOS              *os2.OS
		osExecCommandsMock    *os2.MockOsExecCommands
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		osExecCommandsMock = os2.NewMockOsExecCommands(mockCtrl)
		gracefulRebootChannel = make(chan struct{})
		osExecCommandsMock.EXPECT().IsRpmOstreeAvailable().Return(false)
		deviceOS = os2.NewOS(gracefulRebootChannel, osExecCommandsMock)
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	It("should not refresh Upgrade Status change configuration", func() {
		// when
		upgradeStatus := deviceOS.GetUpgradeStatus()

		// then
		Expect(upgradeStatus.CurrentCommitID).To(Equal(os2.UnknownOsImageId))
		Expect(upgradeStatus.LastUpgradeTime).To(BeEmpty())
		Expect(upgradeStatus.LastUpgradeStatus).To(BeEmpty())
	})

	It("should not Update configuration", func() {
		// given
		cfg := createConfig(false, RequestedCommitId, NewHostedUrl)

		// when
		err := deviceOS.Update(cfg)

		// then
		Expect(err).ToNot(HaveOccurred())

		Expect(deviceOS.AutomaticallyUpgrade).To(BeTrue())
		Expect(deviceOS.HostedObjectsURL).To(BeEmpty())
		Expect(deviceOS.RequestedOsCommit).To(Equal(os2.UnknownOsImageId))
	})
})

func createConfig(automaticallyUpgrade bool, commitID string, hostedUrl string) models.DeviceConfigurationMessage {
	cfg := models.DeviceConfigurationMessage{
		Configuration: &models.DeviceConfiguration{Os: &models.OsInformation{
			AutomaticallyUpgrade: automaticallyUpgrade,
			CommitID:             commitID,
			HostedObjectsURL:     hostedUrl,
		}},
		DeviceID:  "",
		Version:   "",
		Workloads: []*models.Workload{},
	}

	return cfg
}

func returnMockRpmOstreeBootedOnlyStatus(checksum string, upgradeTime int) ([]byte, error) {
	deployment := os2.Deployments{
		Checksum:  checksum,
		Timestamp: upgradeTime,
		Booted:    true,
	}
	deployments := []*os2.Deployments{&deployment}
	status := os2.StatusStruct{Deployments: deployments}
	marshStatus, _ := json.Marshal(status)
	return marshStatus, nil

}

func returnMockRpmOstreeStatus(bootedChecksum string, upgradeTime int, unBootableChecksum string) ([]byte, error) {
	deployments := []*os2.Deployments{
		{
			Checksum:  bootedChecksum,
			Timestamp: upgradeTime,
			Booted:    true,
		},
		{
			Checksum:  unBootableChecksum,
			Timestamp: UnBootableUpgradeTime,
			Booted:    false,
		},
	}
	status := os2.StatusStruct{Deployments: deployments}
	marshStatus, _ := json.Marshal(status)
	return marshStatus, nil

}

func returnMockRpmUpdatePreview(commitID string) ([]byte, error) {
	return []byte(commitID), nil
}

func waitForGracefulRebootChannel(deviceOS *os2.OS) {
	go func() {
		<-deviceOS.GracefulRebootChannel

		time.Sleep(time.Second)
		deviceOS.GracefulRebootCompletionChannel <- struct{}{}

	}()
}
