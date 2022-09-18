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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/apenella/go-ansible/pkg/execute"
	"github.com/apenella/go-ansible/pkg/options"
	"github.com/apenella/go-ansible/pkg/playbook"
	"github.com/apenella/go-ansible/pkg/stdoutcallback"
	ansibleResults "github.com/apenella/go-ansible/pkg/stdoutcallback/results"
	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/api"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/dispatcher"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/mapping"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/model/message"
	"github.com/project-flotta/flotta-device-worker/internal/registration"
	"github.com/project-flotta/flotta-operator/models"
	log "github.com/sirupsen/logrus"

	cfg "github.com/project-flotta/flotta-device-worker/internal/configuration"
	pb "github.com/redhatinsights/yggdrasil/protocol"
)

// Manager handle ansible playbook execution
type Manager struct {
	configManager         *cfg.Manager
	reg                   *registration.Registration
	deviceId              string
	wg                    sync.WaitGroup
	managementLock        sync.Locker
	sendLock              sync.Mutex
	dispatcherClient      pb.DispatcherClient
	ansibleDispatcher     *dispatcher.AnsibleDispatcher
	MappingRepository     mapping.MappingRepository
	eventsQueue           []*models.EventInfo
	tickerLock            sync.RWMutex
	ticker                *time.Ticker
	previousPeriodSeconds int64
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

func NewAnsibleManager(
	configManager *cfg.Manager,
	dispatcherClient pb.DispatcherClient,
	configDir string,
	deviceId string,
	reg registration.RegistrationWrapper) (*Manager, error) {
	mappingRepository, err := mapping.NewMappingRepository(configDir)
	if err != nil {
		return nil, fmt.Errorf("ansible manager cannot initialize mapping repository: %w", err)
	}

	_, err = exec.LookPath("ansible")
	if err != nil {
		return nil, fmt.Errorf("flotta agent requires the ansible package to be installed")
	}

	return &Manager{
		deviceId:          deviceId,
		configManager:     configManager,
		wg:                sync.WaitGroup{},
		managementLock:    &sync.Mutex{},
		dispatcherClient:  dispatcherClient,
		ansibleDispatcher: dispatcher.NewAnsibleDispatcher(deviceId),
		MappingRepository: mappingRepository,
		ticker:            nil,
	}, nil
}

// Init no-op due to we need to an update from the source of truth in this
// case(API)
func (a *Manager) Init(config models.DeviceConfigurationMessage) error {
	return nil
}
func (a *Manager) String() string {
	return "ansible-manager"
}

func (a *Manager) Update(config models.DeviceConfigurationMessage) error {
	periodSeconds := a.getInterval(*config.Configuration)
	previousPeriodSeconds := atomic.LoadInt64(&a.previousPeriodSeconds)
	if previousPeriodSeconds <= 0 || previousPeriodSeconds != periodSeconds {
		log.Debugf("Ansible manager configuration update: periodSeconds changed from %d to %d; Device ID: %s", previousPeriodSeconds, periodSeconds, a.deviceId)
		log.Infof("reconfiguring ticker with interval: %v. DeviceID: %s", periodSeconds, a.deviceId)
		a.stopTicker()

		atomic.StoreInt64(&a.previousPeriodSeconds, periodSeconds)

		a.initTicker(periodSeconds)
		return nil
	}
	return nil
}

func (a *Manager) initTicker(periodSeconds int64) {
	ticker := time.NewTicker(time.Second * time.Duration(periodSeconds))
	a.tickerLock.Lock()
	defer a.tickerLock.Unlock()
	a.ticker = ticker
	go func() {
		for range ticker.C {
			err := a.pushInformation()
			if err != nil {
				log.Errorf("ansible manager interval cannot send the data. DeviceID: %s; err: %s", a.deviceId, err)
			}
		}
	}()

	log.Infof("the ansible manager ticker was started. DeviceID: %s", a.deviceId)
}

func (a *Manager) Start() {
	a.previousPeriodSeconds = a.getInterval(a.configManager.GetDeviceConfiguration())
	a.initTicker(a.getInterval(a.configManager.GetDeviceConfiguration()))
}

func (a *Manager) getInterval(config models.DeviceConfiguration) int64 {
	var interval int64 = 60

	if config.AnsibleManager != nil {
		interval = config.AnsibleManager.PeriodSeconds
	}
	if interval <= 0 {
		interval = 60
	}
	return interval
}

func (a *Manager) HasStarted() bool {
	a.tickerLock.RLock()
	defer a.tickerLock.RUnlock()
	return a.ticker != nil
}

func (a *Manager) pushInformation() error {
	// Create a data message to send back to the dispatcher.

	deviceId := a.deviceId
	content, err := json.Marshal("")
	if err != nil {
		return err
	}

	data := &pb.Data{
		MessageId: uuid.New().String(),
		Content:   []byte(""),
		Directive: "ansible",
	}
	log.Debugf("pushInformation: sending content %+v; DeviceID: %s;", content, deviceId)
	return a.send(data)
}

func (a *Manager) send(data *pb.Data) error {
	a.sendLock.Lock()
	defer a.sendLock.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	log.Debugf("Andible Manager send: Sending data: %+v; Device ID: %s", data, a.deviceId)
	response, err := a.dispatcherClient.Send(ctx, data)
	log.Debugf("Ansible manager send: Response: %+v, err: %+v; Device ID: %s", response, err, a.deviceId)
	if err != nil {
		return err
	}

	parsedResponse, err := NewYGGDResponse(response.Response)
	if err != nil {
		log.Error("cannot transform to NewYGGDResponse %s  err: %v", string(response.Response), err)
		return fmt.Errorf("cannot transform to NewYGGDResponse : %v", err)
	}

	if parsedResponse.StatusCode >= 300 {
		return fmt.Errorf("cannot send ansible message to the operator, status_code=%d, body=%s", parsedResponse.StatusCode, parsedResponse.Body)
	}

	var message models.MessageResponse
	err = json.Unmarshal(parsedResponse.Body, &message)
	if err != nil {
		log.Errorf("cannot unmarshal pe response content: %v", err)
		return fmt.Errorf("cannot unmarshal pe response content: %v", err)
	}

	msgContent := message.Content.(string)
	var parsedContent []*models.PlaybookExecution
	err = json.Unmarshal([]byte(msgContent), &parsedContent)

	if err != nil {
		log.Errorf(">> cannot parse message content %v. Error: %v", message.Content, err)
		return fmt.Errorf("cannot parse message content")
	}

	var errors error
	for _, p := range parsedContent {
		fmt.Printf("STRUCT ELEM %+v\n", p)

		playbookCmd := a.GetPlaybookCommand()
		timeout := getTimeout(parsedResponse.Metadata)

		err = a.HandlePlaybook(message.MessageID, message.Metadata.(map[string]interface{}), p.Name, p.AnsiblePlaybookString, playbookCmd, timeout)

		if err != nil {
			log.Errorf("cannot handle playbook execution %v. %v", p, err)
			errors = multierror.Append(errors, err)
		}
	}
	return errors
}

type YGGDResponse struct {
	Metadata   map[string]string
	Body       json.RawMessage
	StatusCode int
}

func NewYGGDResponse(response []byte) (*YGGDResponse, error) {
	var parsedResponse YGGDResponse
	err := json.Unmarshal(response, &parsedResponse)
	if err != nil {
		log.Warnf("NewYGGDResponse Error unmarshal %s", string(response))
		return nil, err
	}
	return &parsedResponse, nil
}

func (a *Manager) parsePlaybookExecutionResponse(response []byte) (*YGGDResponse, error) {
	parsedResponse, err := NewYGGDResponse(response)
	if err != nil {
		return nil, err
	}

	// TODO
	// if parsedResponse.StatusCode >= 300 {

	// 	return fmt.Errorf("cannot register to the operator, status_code=%d, body=%s", parsedResponse.StatusCode, parsedResponse.Body)
	// }

	var message models.MessageResponse
	err = json.Unmarshal(parsedResponse.Body, &message)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshal playbook execution response content: %v", err)
	}

	// parsedContent := []*models.PlaybookExecution{}
	// var parsedContent interface{}
	parsedContent, ok := message.Content.(map[string]interface{})

	if !ok { //err != nil {
		log.Errorf("cannot parse message content. parsedContent: %v    err: %v", parsedContent, err)
		return nil, fmt.Errorf("cannot parse message content")
	}

	log.Debugf("@@@ Check parsed content %v", parsedContent)
	// cert, ok := parsedContent["certificate"]
	// if !ok {
	// 	return fmt.Errorf("cannot retrieve certificate from parsedResponse")
	// }

	// parsedCert, ok := cert.(string)
	// if !ok {
	// 	return fmt.Errorf("cannot parse certificate from response.Content, content=%+v", message.Content)
	// }

	// err = r.clientCert.WriteCertificate([]byte(parsedCert), key)
	// if err != nil {
	// 	log.Errorf("failed to write certificate: %v,", err)
	// 	return err
	// }

	return parsedResponse, nil
}

func getTimeout(metadata map[string]string) time.Duration {
	timeout := 300 * time.Second // Deafult timeout
	if timeoutStr, found := metadata["ansible-playbook-timeout"]; found {
		timeoutVal, err := strconv.Atoi(fmt.Sprintf("%v", timeoutStr))
		if err != nil {
			log.Warnf("invalid timeout received %s. Use default of %s", timeoutStr, timeout)
			return timeout
		}
		timeout = time.Duration(timeoutVal) * time.Second
		log.Infof("set timeout to %s", timeout)
	}
	return timeout
}

func (a *Manager) Deregister() error {
	log.Infof("stopping ansible manager ticker. DeviceID: %s", a.deviceId)
	a.stopTicker()
	return nil
}

func (a *Manager) stopTicker() {
	if a.HasStarted() {
		a.tickerLock.RLock()
		defer a.tickerLock.RUnlock()
		a.ticker.Stop()
	}
}

func (a *Manager) List() []api.PlaybookExecutionInfo {
	nameStatuses := a.MappingRepository.GetAllNamesStatus()
	infos := make([]api.PlaybookExecutionInfo, len(nameStatuses))
	i := 0
	for name, status := range nameStatuses {
		infos[i] = api.PlaybookExecutionInfo{
			Name:   name,
			Status: status,
		}
		i = i + 1
	}
	return infos
}

func isResponseEmpty(response *pb.Response) bool {
	return response == nil || len(response.Response) == 0
}

func MissingAttributeError(attribute string, metadata map[string]interface{}) error {
	return fmt.Errorf(missingAttributeMsg(attribute, metadata))
}

func missingAttributeMsg(attribute string, metadata map[string]interface{}) string {
	return fmt.Sprintf("missing attribute %s in message metadata %+v", attribute, metadata)
}

// Set executor and stdoutcallback
func setupPlaybookCmd(playbookCmd playbook.AnsiblePlaybookCmd, buffOut, buffErr *bytes.Buffer) playbook.AnsiblePlaybookCmd {
	playbookExecutor := execute.NewDefaultExecute(
		execute.WithWrite(io.Writer(buffOut)),
		execute.WithWriteError(io.Writer(buffErr)),
	)

	playbookCmd.Exec = playbookExecutor
	return playbookCmd
}

func (a *Manager) HandlePlaybook(messageId string, metadataMap map[string]interface{}, peName string, playbookString string, playbookCmd playbook.AnsiblePlaybookCmd, timeout time.Duration) error {
	var err error
	buffOut := new(bytes.Buffer)
	buffErr := new(bytes.Buffer)

	playbookCmd = setupPlaybookCmd(playbookCmd, buffOut, buffErr)

	// deviceConfigurationMessage := models.DeviceConfigurationMessage{}

	if len(playbookString) == 0 {
		return fmt.Errorf("empty message. messageID: %s", messageId)
	}
	// err = json.Unmarshal([]byte(playbookString), &deviceConfigurationMessage)
	// if err != nil {
	// 	log.Error("Error while converting message content to map ", err)
	// }

	// required fields
	reqFields := &RequiredFields{}
	// payloadStr := deviceConfigurationMessage.AnsiblePlaybook
	log.Infof("Handle Playbook Content message: %s", playbookString)
	found := false

	responseTo := messageId
	if playbookString == "" {
		return fmt.Errorf("missing playbook string in message with messageID: %s", messageId)
	}

	if _, found := metadataMap[crcDispatcherAttribute]; !found {
		return MissingAttributeError(crcDispatcherAttribute, metadataMap)
	}

	var ok bool
	if reqFields.crcDispatcherCorrelationID, ok = metadataMap[crcDispatcherAttribute].(string); !ok {
		return fmt.Errorf("cannot assert type of crc id. Metadata map: %+v", metadataMap)
	}

	if _, found = metadataMap[returnURLAttribute]; !found {
		return fmt.Errorf(missingAttributeMsg(returnURLAttribute, metadataMap))
	}

	if reqFields.crcDispatcherCorrelationID, ok = metadataMap[returnURLAttribute].(string); !ok {
		return fmt.Errorf("cannot assert type of crc id. Metadata map: %+v", metadataMap)
	}

	playbookYamlFile := path.Join(dataDir, "ansible_playbook_"+messageId+".yml")
	err = os.WriteFile(playbookYamlFile, []byte(playbookString), 0600)
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
	if err != nil {
		return fmt.Errorf("cannot stat file %s", playbookYamlFile)
	}
	a.MappingRepository.Add(peName, []byte(playbookCmd.Playbooks[0]), fileInfo.ModTime(), "Deploying")
	// execute
	a.wg.Add(1)
	go execPlaybook(peName, executionCompleted, playbookResults, playbookCmd, timeout, reqFields.returnURL, buffOut, a.MappingRepository, fileInfo.Name(), fileInfo.ModTime())
	var errRunPlaybook error
	for {
		select {
		case <-ctx.Done():
			a.wg.Done()
			return fmt.Errorf("execution timeout reached for playbook in messageID %s. Error: %v", messageId, ctx.Err())
		case errRunPlaybook = <-executionCompleted:
			a.wg.Done()
			log.Infof("ansible playbook execution completed of messageID %s", messageId)
			if errRunPlaybook != nil {
				log.Errorf("ansible playbook execution completed with error. [MessageID: %s, Error: %v]", messageId, errRunPlaybook)
				// last event should be the failure, find the reason
				msgList := a.ansibleDispatcher.GetMsgList()
				errorCode, errorDetails := parseFailure(msgList[len(msgList)-1])
				// required event for cloud connector
				onFailed := a.ansibleDispatcher.ExecutorOnFailed(reqFields.crcDispatcherCorrelationID, "", errorCode, fmt.Sprintf("%v", errorDetails))
				a.ansibleDispatcher.AddRunnerJobEvent(onFailed)
			}
			return nil
		case results := <-playbookResults:
			log.Debugf("posting events for messageID %s", messageId)
			err := a.sendEvents(results, reqFields.returnURL, responseTo, playbookYamlFile)
			if err != nil {
				log.Errorf("cannot post ansible playbook results of message %s: %v", messageId, err)
			}
		}
	}
}

// sendEvents adds the events of AnsiblePlaybookJSONResults into eventList and sends them to the dispatcher.
func (a *Manager) sendEvents(results *ansibleResults.AnsiblePlaybookJSONResults, returnURL string, responseTo string, playbookYamlFile string) error {
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

func (a *Manager) ExecutePendingPlaybooks() error {
	timeout := 300 * time.Second // Deafult timeout

	playbookCmd := a.GetPlaybookCommand()
	buffOut := new(bytes.Buffer)
	buffErr := new(bytes.Buffer)

	playbookCmd = setupPlaybookCmd(playbookCmd, buffOut, buffErr)
	allPlaybooks := a.MappingRepository.GetAll()
	var errors error
	for _, v := range allPlaybooks {
		playbookCmd.Playbooks = []string{v}
		res, err := a.execPlaybookSync(&playbookCmd, timeout, buffOut, a.MappingRepository)
		if err != nil {
			log.Error(err)
			errors = multierror.Append(errors, err)
		}

		if res == nil && err != nil {
			return err
		}
		buffOut.Reset()
		buffErr.Reset()
	}
	if errors != nil {
		a.AddToEventQueue(&models.EventInfo{
			Message: errors.Error(),
			Reason:  "Failed",
			Type:    models.EventInfoTypeWarn,
		})
	}
	return errors
}

func (a *Manager) AddToEventQueue(event *models.EventInfo) {
	a.eventsQueue = append(a.eventsQueue, event)
}

func (a *Manager) WaitPlaybookCompletion() {
	a.wg.Wait()
}

func (a *Manager) GetPlaybookCommand() playbook.AnsiblePlaybookCmd {
	// defined how to connect to hosts
	ansiblePlaybookConnectionOptions := &options.AnsibleConnectionOptions{
		Connection: "local",
	}
	// defined which should be the ansible-playbook execution behavior and where to find execution configuration.
	ansiblePlaybookOptions := &playbook.AnsiblePlaybookOptions{
		Inventory: "127.0.0.1,",
	}

	return playbook.AnsiblePlaybookCmd{
		ConnectionOptions: ansiblePlaybookConnectionOptions,
		Options:           ansiblePlaybookOptions,
		StdoutCallback:    stdoutcallback.JSONStdoutCallback,
	}

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
	peName string, //PlaybookExecution name
	executionCompleted chan error,
	playbookResults chan<- *ansibleResults.AnsiblePlaybookJSONResults,
	playbookCmd playbook.AnsiblePlaybookCmd,
	timeout time.Duration,
	messageID string,
	buffOut *bytes.Buffer,
	mappingRepository mapping.MappingRepository,
	fileName string,
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

	fileContent, err := os.ReadFile(fileName) //TODO: handle more than the first playbook
	if err != nil {
		executionCompleted <- err
		return
	}

	// err = mappingRepository.Add(peName, fileContent, modTime, "Deploying")
	// if err != nil {
	// 	executionCompleted <- err
	// 	return
	// }
	mappingRepository.UpdateStatus(peName, "Running")
	errRun := playbookCmd.Run(ctx)

	if errRun != nil {
		log.Warnf("playbook executed with errors. Results: %s, messageID: %s, Error: %v", buffOut.String(), messageID, errRun)
		mappingRepository.UpdateStatus(peName, "CompletedWithError")
	} else {
		mappingRepository.UpdateStatus(peName, "SuccessfullyCompleted")
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

	err = mappingRepository.Remove(fileContent)
	if err != nil {
		log.Errorf("cannot remove pending playbook %s. MessageID: %s, Error: %v\n", string(fileContent), messageID, err)
	}

	// Signal that the playbook execution completed
	executionCompleted <- errRun
}

// execPlaybookSync executes the ansible playbook synchronously.
// if error is not nil, then an error occured during the playbook execution (e.g. host unreachable), but this does't mean
// that there are no results available. In other words, a successful call returns *ansibleResults.AnsiblePlaybookJSONResults not nil.
func (a *Manager) execPlaybookSync(
	playbook *playbook.AnsiblePlaybookCmd,
	timeout time.Duration,
	buffOut *bytes.Buffer,
	mappingRepository mapping.MappingRepository) (*ansibleResults.AnsiblePlaybookJSONResults, error) {

	fileContent, err := os.ReadFile(playbook.Playbooks[0])
	if err != nil {
		log.Errorf("cannot read pending playbook file %s", playbook.Playbooks[0])
		return nil, err
	}

	log.Debugf("Executing %v", playbook.Playbooks)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	errRun := playbook.Run(ctx)

	if errRun != nil {
		log.Warnf("pending playbook executed with errors. Results: %s, playbookFile: %v, Error: %v", buffOut.String(), playbook.Playbooks, errRun)
	}
	results, err := ansibleResults.JSONParse(buffOut.Bytes())

	if err != nil {
		log.Errorf("error while parsing json string %s. MessageID: %v, Error: %v\n", buffOut.String(), playbook.Playbooks, err)
		// Signal that the playbook execution completed with error
		return nil, err
	}

	err = mappingRepository.Remove(fileContent)
	if err != nil {
		log.Errorf("cannot remove pending playbook %s. Error: %v\n", string(fileContent), err)
	}

	return results, errRun
}

// PopEvents return copy of all the events stored in eventQueue
func (a *Manager) PopEvents() []*models.EventInfo {
	a.managementLock.Lock()
	defer a.managementLock.Unlock()

	// Copy the events:
	events := []*models.EventInfo{}
	for _, event := range a.eventsQueue {
		e := *event
		events = append(events, &e)
	}
	// Empty the events:
	a.eventsQueue = []*models.EventInfo{}
	return events
}
