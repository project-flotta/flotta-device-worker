package ansible

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/apenella/go-ansible/pkg/execute"
	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"
	ansibleResults "github.com/apenella/go-ansible/pkg/stdoutcallback/results"
	"github.com/hashicorp/go-multierror"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/dispatcher"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/mapping"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/model/message"
	"github.com/project-flotta/flotta-operator/models"

	pb "github.com/redhatinsights/yggdrasil/protocol"
)

// AnsibleManager handle ansible playbook execution
type AnsibleManager struct {
	wg                *sync.WaitGroup
	dispatcherClient  pb.DispatcherClient
	ansibleDispatcher *dispatcher.AnsibleDispatcher
	MappingRepository mapping.MappingRepository
}

type RequiredFields struct {
	crcDispatcherCorrelationID, returnURL string
}

const (

	//Required field names
	crcDispatcherAttribute = "crc_dispatcher_correlation_id"
	returnURLAttribute     = "return_url"

	// Failure types
	NotInstalled = "ANSIBLE_NOT_INSTALLED"
	UndefError   = "UNDEFINED_ERROR"
	dataDir      = "/tmp"
)

var (
	deviceID string
)

func NewAnsibleManager(
	dispatcherClient pb.DispatcherClient, configDir string) (*AnsibleManager, error) {
	mappingRepository, err := mapping.NewMappingRepository(configDir)
	if err != nil {
		return nil, fmt.Errorf("ansible manager cannot initialize mapping repository: %w", err)
	}

	_, err = exec.LookPath("ansible")
	if err != nil {
		return nil, fmt.Errorf("flotta agent requires the ansible package to be installed")
	}

	return &AnsibleManager{
		wg:                &sync.WaitGroup{},
		dispatcherClient:  dispatcherClient,
		ansibleDispatcher: dispatcher.NewAnsibleDispatcher(deviceID),
		MappingRepository: mappingRepository,
	}, nil
}

func MissingAttributeError(attribute string, metadata map[string]string) error {
	return fmt.Errorf(missingAttributeMsg(attribute, metadata))
}

func missingAttributeMsg(attribute string, metadata map[string]string) string {
	return fmt.Sprintf("missing attribute %s in message metadata %+v", attribute, metadata)
}

// Set executor and stdoutcallback
func setupPlaybookCmd(playbookCmd *playbook.AnsiblePlaybookCmd, buffOut, buffErr *bytes.Buffer) {
	playbookExecutor := execute.NewDefaultExecute(
		execute.WithWrite(io.Writer(buffOut)),
		execute.WithWriteError(io.Writer(buffErr)),
	)

	playbookCmd.Exec = playbookExecutor
	playbookCmd.StdoutCallback = "json"
}

func (a *AnsibleManager) HandlePlaybook(playbookCmd *playbook.AnsiblePlaybookCmd, d *pb.Data, timeout time.Duration) error {
	var err error
	buffOut := new(bytes.Buffer)
	buffErr := new(bytes.Buffer)

	setupPlaybookCmd(playbookCmd, buffOut, buffErr)

	deviceConfigurationMessage := models.DeviceConfigurationMessage{}

	if string(d.Content) == "" {
		return fmt.Errorf("empty message. messageID: %s", d.MessageId)
	}
	err = json.Unmarshal(d.Content, &deviceConfigurationMessage)
	if err != nil {
		log.Error("Error while converting message content to map ", err)
	}

	// required fields
	reqFields := &RequiredFields{}
	payloadStr := deviceConfigurationMessage.AnsiblePlaybook
	log.Infof("Handle Playbook Content message: %s", payloadStr)
	found := false

	responseTo := d.MessageId
	if payloadStr == "" {
		return fmt.Errorf("missing playbook string in message with messageID: %s", d.MessageId)
	}
	metadataMap := d.GetMetadata()
	if reqFields.crcDispatcherCorrelationID, found = metadataMap[crcDispatcherAttribute]; !found {
		return MissingAttributeError(crcDispatcherAttribute, metadataMap)
	}

	if reqFields.returnURL, found = metadataMap[returnURLAttribute]; !found {
		return fmt.Errorf(missingAttributeMsg(returnURLAttribute, metadataMap))
	}
	playbookYamlFile := path.Join(dataDir, "ansible_playbook_"+d.MessageId+".yml")
	err = os.WriteFile(playbookYamlFile, []byte(payloadStr), 0600)
	if err != nil {
		return fmt.Errorf("cannot create ansible playbook yaml file %s. Error: %v", playbookYamlFile, err)
	}
	defer os.Remove(playbookYamlFile)

	/*
	*******************************************
	*  TODO : Verify playbook
	*******************************************
	 */

	// required event for cloud connector
	onStart := a.ansibleDispatcher.ExecutorOnStart(reqFields.crcDispatcherCorrelationID, "")
	a.ansibleDispatcher.AddRunnerJobEvent(onStart)

	executionCompleted := make(chan error)
	playbookResults := make(chan *ansibleResults.AnsiblePlaybookJSONResults)
	playbookCmd.Playbooks = []string{playbookYamlFile}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	fileInfo, err := os.Stat(playbookYamlFile)
	// execute
	a.wg.Add(1)
	go execPlaybook(executionCompleted, playbookResults, playbookCmd, timeout, reqFields.returnURL, buffOut, a.MappingRepository, fileInfo.ModTime())
	var errRunPlaybook error
loop:
	for {
		select {
		case <-ctx.Done():
			a.wg.Done()
			return fmt.Errorf("execution timeout reached for playbook in messageID %s. Error: %v", d.MessageId, ctx.Err())
		case errRunPlaybook = <-executionCompleted:
			a.wg.Done()
			if err != nil {
				return fmt.Errorf("ansible playbook execution completed with error. [MessageID: %s, Error: %v]", d.MessageId, err)
			}
			log.Infof("ansible playbook execution completed of messageID %s", d.MessageId)
			break loop

		case results := <-playbookResults:
			log.Debugf("posting events for messageID %s", d.MessageId)
			err := a.sendEvents(results, reqFields.returnURL, responseTo, playbookYamlFile)
			if err != nil {
				log.Errorf("cannot post ansible playbook results of message %s: %v", d.MessageId, err)
			}
		}
	}
}

// sendEvents adds the events of AnsiblePlaybookJSONResults into eventList and sends them to the dispatcher.
func (a *AnsibleManager) sendEvents(results *ansibleResults.AnsiblePlaybookJSONResults, returnURL string, responseTo string, playbookYamlFile string) error {
	if results == nil || results.Plays == nil {
		err := fmt.Errorf("cannot compose empty message for %s", responseTo)
		log.Error(err)
		return err
	}
	eventList := a.ansibleDispatcher.AddEvent(playbookYamlFile, results)
	message, err := dispatcher.ComposeDispatcherMessage(eventList, returnURL, responseTo)
	if err != nil {
		log.Errorf("cannot compose message for events: %v. ResponseTo: %s, Error: %v", eventList, responseTo, err)
		return err
	}
	log.Infof("Message to be sent as reply: %v", string(message.Content))
	_, err = a.dispatcherClient.Send(context.Background(), message)
	if err != nil {
		log.Errorf("cannot send message %s to the dispatcher. ResponseTo: %s, Content: %s, Error: %v", message.MessageId, responseTo, string(message.Content), err)
		return err
	}
	return nil
}

func (a *AnsibleManager) ExecutePendingPlaybooks() error {
	timeout := 300 * time.Second // Deafult timeout

	// defined how to connect to hosts
	ansiblePlaybookConnectionOptions := &options.AnsibleConnectionOptions{
		Connection: "local",
	}
	// defined which should be the ansible-playbook execution behavior and where to find execution configuration.
	ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
		Inventory: "127.0.0.1,",
	}

	playbookCmd := &playbook.AnsiblePlaybookCmd{
		ConnectionOptions: ansiblePlaybookConnectionOptions,
		Options:           ansiblePlaybookOptions,
	}

	buffOut := new(bytes.Buffer)
	buffErr := new(bytes.Buffer)

	setupPlaybookCmd(playbookCmd, buffOut, buffErr)
	allPlaybooks := a.MappingRepository.GetAll()
	var errors error
	for _, v := range allPlaybooks {
		playbookCmd.Playbooks = []string{v}
		res, err := a.execPlaybookSync(playbookCmd, timeout, buffOut, a.MappingRepository)
		log.Error(err)
		errors = multierror.Append(errors, err)
		if res == nil && err != nil {
			return err
		}
		buffOut.Reset()
		buffErr.Reset()
	}
	return errors
}

func (a *AnsibleManager) StopPlaybooks() {
	a.wg.Wait()
}

// parseFailure generates the error code and details from the failure event
func parseFailure(event message.AnsibleRunnerJobEventYaml) (errorCode string, errorDetails interface{}) {
	errorCode = UndefError
	errorDetails = event.Stdout
	if strings.Contains(fmt.Sprintf("%v", errorDetails), "The command was not found or was not executable: ansible-playbook") {
		errorCode = NotInstalled
	}
	// TODO: enumerate more failure types
	return errorCode, errorDetails
}

// execPlaybook executes the ansible playbook.
// It sends ansible playbook results when the playbook execution has been completed on playbookResults channel.
// When the execution terminates, execPlaybook signals on executionCompleted channel if there was an error.
func execPlaybook(
	executionCompleted chan error,
	playbookResults chan<- *ansibleResults.AnsiblePlaybookJSONResults,
	playbook *playbook.AnsiblePlaybookCmd,
	timeout time.Duration,
	messageID string,
	buffOut *bytes.Buffer,
	mappingRepository mapping.MappingRepository,
	modTime time.Time) {

	defer close(executionCompleted)
	defer close(playbookResults)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Check if the playbook has been already executed on the device.
	alreadyExecuted := mappingRepository.Exists(modTime)

	if alreadyExecuted {
		err := fmt.Errorf("playbook of messageID %s is already in execution", messageID)
		executionCompleted <- err
		return
	}

	fileContent, err := os.ReadFile(playbook.Playbooks[0])
	if err != nil {
		executionCompleted <- err
		return
	}

	err = mappingRepository.Add(string(fileContent), modTime)
	if err != nil {
		executionCompleted <- err
		return
	}
	errRun := playbook.Run(ctx)

	if errRun != nil {
		log.Warnf("playbook executed with errors. Results: %s, messageID: %s, Error: %v", buffOut.String(), messageID, errRun)
	}
	results, err := ansibleResults.JSONParse(buffOut.Bytes())

	if err != nil {
		log.Errorf("error while parsing json string %s. MessageID: %s, Error: %v\n", buffOut.String(), messageID, err)
		// Signal that the playbook execution completed with error
		executionCompleted <- err
		// No more work to be done, return
		return
	}

	playbookResults <- results

	// Signal that the playbook execution completed
	executionCompleted <- errRun
}

// execPlaybookSync executes the ansible playbook synchronously.
// if error is not nil, then an error occured during the playbook execution (e.g. host unreachable), but this does't mean
// that there are no results available. In other words, a successful call returns *ansibleResults.AnsiblePlaybookJSONResults not nil.
func (a *AnsibleManager) execPlaybookSync(
	playbook *playbook.AnsiblePlaybookCmd,
	timeout time.Duration,
	buffOut *bytes.Buffer,
	mappingRepository mapping.MappingRepository) (*ansibleResults.AnsiblePlaybookJSONResults, error) {

	log.Debugf("Executing %v", playbook.Playbooks)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	errRun := playbook.Run(ctx)

	if errRun != nil {
		log.Warnf("playbook executed with errors. Results: %s, playbookFile: %v, Error: %v", buffOut.String(), playbook.Playbooks, errRun)
	}
	results, err := ansibleResults.JSONParse(buffOut.Bytes())

	if err != nil {
		log.Errorf("error while parsing json string %s. MessageID: %v, Error: %v\n", buffOut.String(), playbook.Playbooks, err)
		// Signal that the playbook execution completed with error
		return nil, err
	}

	return results, errRun
}
