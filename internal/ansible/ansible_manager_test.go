package ansible_test

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"time"

	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/project-flotta/flotta-device-worker/internal/ansible"
	pb "github.com/redhatinsights/yggdrasil/protocol"
	"google.golang.org/grpc"
)

var _ = Describe("Ansible Runner", func() {
	const (
		returnUrl = "test_return_url"
		configDir = "/tmp/testAnsible/"
	)
	var (
		messageID      string
		err            error
		ansibleManager *ansible.AnsibleManager
		client         Dispatcher
		timeout        time.Duration

		playbookCmd playbook.AnsiblePlaybookCmd
	)

	BeforeEach(func() {
		client = Dispatcher{}
		messageID = "msg_" + uuid.New().String()

		ansibleManager, err = ansible.NewAnsibleManager(&client, configDir)
		playbookCmd = ansibleManager.GetPlaybookCommand()
		Expect(err).ToNot(HaveOccurred())

		timeout = 600 * time.Second
		err = os.RemoveAll(configDir)
		Expect(err).ToNot(HaveOccurred())
		err = os.Mkdir(configDir, 0777)
		Expect(err).ToNot(HaveOccurred())
	})
	AfterEach(func() {
		err = ansibleManager.MappingRepository.RemoveMappingFile()
		Expect(err).ToNot(HaveOccurred())
	})
	Context("Execute playbook", func() {
		It("Empty playbook", func() {
			//given
			samplePlaybook := ""
			m := map[string]string{"ansible-playbook": "true", "crc_dispatcher_correlation_id": "d4ae95cf-71fd-4386-8dbf-2bce933ce713", "response_interval": "250", "return_url": returnUrl}
			message := pb.Data{
				MessageId: messageID,
				Metadata:  m,
				Content:   []byte(samplePlaybook),
			}

			//when
			err = ansibleManager.HandlePlaybook(&playbookCmd, &message, timeout)

			//then
			Expect(err).To(HaveOccurred(), "missing playbook string in message")
		})
		It("Run successfully an ansible playbook ", func() {

			//given
			playbookFilename := "examples/test_playbook_1.yml"
			playbookCmd.Playbooks = []string{playbookFilename}

			ansibleManager, err = ansible.NewAnsibleManager(&client, configDir)
			Expect(err).ToNot(HaveOccurred())

			playbookContent, err := os.ReadFile(playbookFilename)
			if err != nil {
				Fail("cannot read playbook test file: " + playbookFilename)
			}

			m := map[string]string{"ansible-playbook": "true", "crc_dispatcher_correlation_id": "d4ae95cf-71fd-4386-8dbf-2bce933ce713", "response_interval": "250", "return_url": returnUrl}
			msgContent := map[string]string{"ansible_playbook": string(playbookContent)}
			msgContentByte, err := json.Marshal(msgContent)
			if err != nil {
				Fail("cannot crete message content")
			}
			message := pb.Data{
				MessageId: messageID,
				Metadata:  m,
				Content:   msgContentByte,
			}

			//when
			err = ansibleManager.HandlePlaybook(&playbookCmd, &message, timeout)
			Expect(err).ToNot(HaveOccurred())
			//then
			Expect(client.latestData).ToNot(BeNil())
			Expect(client.latestData.Directive).To(Equal(returnUrl))

		})
		It("Test ansible playbook ", func() {

			//given
			playbookFilename := "examples/test_playbook_2.yml"
			playbookCmd.Playbooks = []string{playbookFilename}

			ansibleManager, err := ansible.NewAnsibleManager(&client, configDir)
			Expect(err).ToNot(HaveOccurred())

			playbookContent, err := os.ReadFile(playbookFilename)
			if err != nil {
				Fail("cannot read playbook test file: " + playbookFilename)
			}

			m := map[string]string{"ansible-playbook": "true", "crc_dispatcher_correlation_id": "d4ae95cf-71fd-4386-8dbf-2bce933ce713", "response_interval": "250", "return_url": returnUrl}
			msgContent := map[string]string{"ansible_playbook": string(playbookContent)}
			msgContentByte, err := json.Marshal(msgContent)
			if err != nil {
				Fail("cannot crete message content")
			}
			message := pb.Data{
				MessageId: messageID,
				Metadata:  m,
				Content:   msgContentByte,
			}

			//when
			err = ansibleManager.HandlePlaybook(&playbookCmd, &message, timeout)
			Expect(err).ToNot(HaveOccurred())
			Expect(client.latestData.Directive).To(Equal(returnUrl))
		})
		It("Execute slow ansible playbook", func() {

			timeout = 1 * time.Millisecond
			//given
			playbookFilename := "examples/test_playbook_timeout.yml"
			playbookCmd.Playbooks = []string{playbookFilename}

			ansibleManager, err := ansible.NewAnsibleManager(&client, configDir)
			Expect(err).ToNot(HaveOccurred())

			playbookContent, err := os.ReadFile(playbookFilename)
			if err != nil {
				Fail("cannot read playbook test file: " + playbookFilename)
			}

			m := map[string]string{"ansible-playbook": "true", "crc_dispatcher_correlation_id": "d4ae95cf-71fd-4386-8dbf-2bce933ce713", "response_interval": "250", "return_url": returnUrl}
			msgContent := map[string]string{"ansible_playbook": string(playbookContent)}
			msgContentByte, err := json.Marshal(msgContent)
			if err != nil {
				Fail("cannot crete message content")
			}
			message := pb.Data{
				MessageId: messageID,
				Metadata:  m,
				Content:   msgContentByte,
			}

			//when
			err = ansibleManager.HandlePlaybook(&playbookCmd, &message, timeout)
			Expect(err).To(HaveOccurred())
			Expect(client.latestData).To(BeNil())
		})
	})
	Context("On restart", func() {
		It("Execute playbook on restart", func() {
			//given
			p1, _ := os.ReadFile("examples/test_playbook_1.yml")
			p2, _ := os.ReadFile("examples/test_playbook_3.yml")

			modTime1 := time.Now().Add(-3 * time.Hour)
			modTime2 := time.Now().Add(-2 * time.Hour)

			p1Sha := ansibleManager.MappingRepository.GetSha256(p1)
			p2Sha := ansibleManager.MappingRepository.GetSha256(p2)

			p1Path := path.Join(configDir, p1Sha)
			p2Path := path.Join(configDir, p2Sha)

			Expect(ansibleManager.MappingRepository.GetAll()).To(BeEmpty())

			err = ansibleManager.MappingRepository.Add(p1, modTime1)
			Expect(err).ToNot(HaveOccurred())
			err = ansibleManager.MappingRepository.Add(p2, modTime2)
			Expect(err).ToNot(HaveOccurred())

			Expect(ansibleManager.MappingRepository.GetAll()).ToNot(BeEmpty())
			Expect(len(ansibleManager.MappingRepository.GetAll())).To(Equal(2))

			//when
			err = ansibleManager.ExecutePendingPlaybooks()
			Expect(err).ToNot(BeNil()) //This is a multierror

			//then
			Expect(ansibleManager.MappingRepository.GetModTime(p1Path)).To(Equal(modTime1.UnixNano()))
			Expect(ansibleManager.MappingRepository.GetModTime(p2Path)).To(Equal(modTime2.UnixNano()))

			Expect(ansibleManager.MappingRepository.GetFilePath(modTime1)).To(Equal(p1Path))
			Expect(ansibleManager.MappingRepository.GetFilePath(modTime2)).To(Equal(p2Path))
		})
	})
})

// We keep the latest send data to make sure that we validate the data sent to
// the operator without sent at all
type Dispatcher struct {
	latestData *pb.Data
}

func (d *Dispatcher) Send(ctx context.Context, in *pb.Data, opts ...grpc.CallOption) (*pb.Response, error) {
	d.latestData = in
	return nil, nil
}

func (d *Dispatcher) Register(ctx context.Context, in *pb.RegistrationRequest, opts ...grpc.CallOption) (*pb.RegistrationResponse, error) {
	return nil, nil
}

func (d *Dispatcher) GetConfig(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.Config, error) {
	return nil, nil
}
