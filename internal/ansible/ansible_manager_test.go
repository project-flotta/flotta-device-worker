package ansible_test

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/apenella/go-ansible/pkg/options"
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
	)
	var (
		messageID      string
		ansibleManager *ansible.AnsibleManager
		client         Dispatcher
		timeout        time.Duration
		// defined how to connect to hosts
		ansiblePlaybookConnectionOptions = &options.AnsibleConnectionOptions{
			Connection: "local",
		}
		// defined which should be the ansible-playbook execution behavior and where to find execution configuration.
		ansiblePlaybookOptions = &playbook.AnsiblePlaybookOptions{
			Inventory: "127.0.0.1,",
		}
		playbookCmd *playbook.AnsiblePlaybookCmd
	)

	BeforeEach(func() {
		client = Dispatcher{}
		messageID = "msg_" + uuid.New().String()

		playbookCmd = &playbook.AnsiblePlaybookCmd{
			ConnectionOptions: ansiblePlaybookConnectionOptions,
			Options:           ansiblePlaybookOptions,
			StdoutCallback:    "json",
		}
		ansibleManager = ansible.NewAnsibleManager(&client)

		timeout = 600 * time.Second

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
			err := ansibleManager.HandlePlaybook(playbookCmd, &message, timeout)

			//then
			Expect(err).To(HaveOccurred(), "missing playbook string in message")
		})
		It("Run successfully an ansible playbook ", func() {

			//given
			playbookFilename := "examples/test_playbook_1.yml"
			playbookCmd = &playbook.AnsiblePlaybookCmd{
				Playbooks:         []string{playbookFilename},
				ConnectionOptions: ansiblePlaybookConnectionOptions,
				Options:           ansiblePlaybookOptions,
			}

			ansibleManager = ansible.NewAnsibleManager(&client)

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
			err = ansibleManager.HandlePlaybook(playbookCmd, &message, timeout)
			Expect(err).Should(BeNil())

			//then
			Expect(client.latestData.Directive).To(Equal(returnUrl))

		})
		It("Test ansible playbook ", func() {

			//given
			playbookFilename := "examples/test_playbook_2.yml"
			playbookCmd = &playbook.AnsiblePlaybookCmd{
				Playbooks:         []string{playbookFilename},
				ConnectionOptions: ansiblePlaybookConnectionOptions,
				Options:           ansiblePlaybookOptions,
			}

			ansibleManager = ansible.NewAnsibleManager(&client)

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
			err = ansibleManager.HandlePlaybook(playbookCmd, &message, timeout)
			Expect(err).Should(BeNil())
			Expect(client.latestData.Directive).To(Equal(returnUrl))
		})
		It("Execute slow ansible playbook", func() {

			timeout = 1 * time.Second
			//given
			playbookFilename := "examples/test_playbook_timeout.yml"
			playbookCmd = &playbook.AnsiblePlaybookCmd{
				Playbooks:         []string{playbookFilename},
				ConnectionOptions: ansiblePlaybookConnectionOptions,
				Options:           ansiblePlaybookOptions,
			}

			ansibleManager = ansible.NewAnsibleManager(&client)

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
			err = ansibleManager.HandlePlaybook(playbookCmd, &message, timeout)
			Expect(err).ShouldNot(BeNil())
			Expect(client.latestData).To(BeNil())
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

func (d *Dispatcher) GetConfig(ctx context.Context, in *pb.Empty, opts ...grpc.CallOption) (*pb.Config, error) {
	return nil, nil
}
