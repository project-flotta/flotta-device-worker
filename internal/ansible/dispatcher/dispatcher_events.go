package dispatcher

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/apenella/go-ansible/pkg/stdoutcallback/results"
	"github.com/google/uuid"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/model/message"
	pb "github.com/redhatinsights/yggdrasil/protocol"
)

// Event-generating functions for the Playbook Dispatcher
type AnsibleDispatcher struct {
	deviceID string
	msgList  []message.AnsibleRunnerJobEventYaml
}

func (d *AnsibleDispatcher) GetMsgList() []message.AnsibleRunnerJobEventYaml {
	return d.msgList
}

func NewAnsibleDispatcher(deviceIDParam string) *AnsibleDispatcher {
	return &AnsibleDispatcher{
		deviceID: deviceIDParam,
		msgList:  []message.AnsibleRunnerJobEventYaml{},
	}
}

//	TODO: Implement this function
func (d *AnsibleDispatcher) AddRunnerJobEvent(event message.AnsibleRunnerJobEventYaml) []message.AnsibleRunnerJobEventYaml {
	d.msgList = append(d.msgList, event)
	return d.msgList
}

func (d *AnsibleDispatcher) AddEvent(playbookFilname string, event *results.AnsiblePlaybookJSONResults) []message.AnsibleRunnerJobEventYaml {
	msgFromEvents := d.eventTransformer(playbookFilname, event)
	d.msgList = append(d.msgList, msgFromEvents...)
	return d.msgList
}

// There is always one and only one v2_playbook_on_start event and it is the first event.
// v2_playbook_on_play_start is generated once per-play in the playbook; two such events
// would be generated from the playbook example below.
// The v2_playbook_on_task_start function is called once for each task under the default execution strategy.
// Other execution strategies (i.e., free or serial) can result in the v2_playbook_on_task_start function being called multiple times, one for each host.
// AWX only creates a Job Event for the first v2_playbook_on_task_start call.
// Subsequent calls for the same task do not result in Job Events being created.
// v2_runner_on_[ok, failed, skipped, unreachable, retry, item_on_ok, item_on_failed, item_on_skipped];
// one v2_runner_on_... Job Event will be created for each v2_playbook_on_task_start event.
func (d *AnsibleDispatcher) eventTransformer(playbookFilename string, events *results.AnsiblePlaybookJSONResults) []message.AnsibleRunnerJobEventYaml {
	var msgList []message.AnsibleRunnerJobEventYaml
	msgCounter := 0
	//playbook_on_play_start is generated once per-play in the playbook
	duplicateTaskCount := make(map[string]int)
	for i := range events.Plays {
		play := events.Plays[i]

		if msgCounter == 1 {
			msgPlaybookStart := d.PlaybookOnStart(playbookFilename, msgCounter, &play)
			msgList = append(msgList, *msgPlaybookStart)
		}

		msgCounter++
		msgPlayStart := d.PlaybookOnPlayStart(playbookFilename, msgCounter, &play)
		msgList = append(msgList, *msgPlayStart)
		taskUUIDs := countOccurence(play.Tasks)

		for _, task := range play.Tasks {
			for _, host := range task.Hosts {
				msgCounter++
				msgTaskOnStart := d.PlaybookOnTaskStart(
					d.deviceID,
					playbookFilename,
					msgPlayStart.Uuid,
					msgCounter,
					msgPlayStart.EventData.Host,
					task.Task,
					host,
					taskUUIDs,
					duplicateTaskCount)
				msgList = append(msgList, *msgTaskOnStart)
				msgCounter++

				//TODO: runner_on_unreachable
				if host.Failed {
					onFailed := d.RunnerOnFailed(d.deviceID, playbookFilename, msgTaskOnStart.Uuid, msgCounter, msgPlayStart.EventData.Host, task.Task, host)
					msgList = append(msgList, *onFailed)
				} else if host.Skipped {
					onSkipped := d.RunnerOnSkipped(d.deviceID, playbookFilename, msgTaskOnStart.Uuid, msgCounter, msgPlayStart.EventData.Host, task.Task, host)
					msgList = append(msgList, *onSkipped)
				} else {
					onOk := d.RunnerOnOk(d.deviceID, playbookFilename, msgTaskOnStart.Uuid, msgCounter, msgPlayStart.EventData.Host,
						task.Task,
						host)
					msgList = append(msgList, *onOk)
				}

			}
		}
		msgCounter++
		msgStats := playbookOnStats(d.deviceID, playbookFilename, msgCounter, play.Play.Duration.End, events.Stats)
		msgList = append(msgList, msgStats)
	}
	return msgList
}

func playbookOnStats(runnerID string, playbookUUID string, msgCounter int, creationTime string,
	stats map[string]*results.AnsiblePlaybookJSONResultsStats,
) message.AnsibleRunnerJobEventYaml {
	message := message.AnsibleRunnerJobEventYaml{
		Event:       "playbook_on_stats",
		RunnerIdent: &runnerID,
		// StartLine:   0, //Not available
		// EndLine:     0, //Not available
		Created: &creationTime,
		Counter: msgCounter,
		EventData: &message.AnsibleRunnerJobEventYamlEventData{
			PlaybookUuid: &playbookUUID,
			Changed:      message.AnsibleRunnerJobEventYamlEventDataChanged{},
			Ok:           message.AnsibleRunnerJobEventYamlEventDataOk{},
			Failures:     message.AnsibleRunnerJobEventYamlEventDataFailures{},
		},
	}
	for host, stats := range stats {
		changesMap := message.EventData.Changed
		changesMap[host] = stats.Changed
		changesOk := message.EventData.Ok
		changesOk[host] = stats.Ok
		changesFailures := message.EventData.Failures
		changesFailures[host] = stats.Failures
	}

	return message
}

func (d *AnsibleDispatcher) RunnerOnOk(
	runnerID string,
	playbookFilename string,
	playbookUUID string,
	msgCounter int,
	host *string,
	tasks *results.AnsiblePlaybookJSONResultsPlayTaskItem,
	hosts *results.AnsiblePlaybookJSONResultsPlayTaskHostsItem) *message.AnsibleRunnerJobEventYaml {
	message := message.AnsibleRunnerJobEventYaml{
		Event:       "v2_runner_on_ok",
		RunnerIdent: &runnerID,
		// StartLine:   0, //Not available
		// EndLine:     0, //Not available
		Created: &tasks.Duration.Start,
		Counter: msgCounter,
		Stdout:  hosts.Stdout,
		EventData: &message.AnsibleRunnerJobEventYamlEventData{
			Playbook:     &playbookFilename,
			PlaybookUuid: &playbookUUID,
			Host:         host,
			TaskUuid:     &tasks.Id,
			Task:         &tasks.Name,
			TaskAction:   &hosts.Action,
			Res: &message.AnsibleRunnerJobEventYamlEventDataRes{
				Changed: &hosts.Changed,
			},
			Uuid: &tasks.Id,
		},
		Uuid: tasks.Id,
	}
	return &message
}

func (d *AnsibleDispatcher) RunnerOnFailed(
	runnerID string,
	playbookFilename string,
	playbookUUID string,
	msgCounter int,
	host *string,
	tasks *results.AnsiblePlaybookJSONResultsPlayTaskItem,
	hosts *results.AnsiblePlaybookJSONResultsPlayTaskHostsItem) *message.AnsibleRunnerJobEventYaml {
	message := message.AnsibleRunnerJobEventYaml{
		Event:       "v2_runner_on_failed",
		RunnerIdent: &runnerID,
		// StartLine:   0, //Not available
		// EndLine:     0, //Not available
		Created: &tasks.Duration.Start,
		Counter: msgCounter,
		Stdout:  hosts.Stdout,
		Stderr:  hosts.Stderr,
		EventData: &message.AnsibleRunnerJobEventYamlEventData{
			Playbook:     &playbookFilename,
			PlaybookUuid: &playbookUUID,
			Host:         host,
			TaskUuid:     &tasks.Id,
			Task:         &tasks.Name,
			TaskAction:   &hosts.Action,
			Res: &message.AnsibleRunnerJobEventYamlEventDataRes{
				Changed: &hosts.Changed,
			},
			Uuid: &tasks.Id,
		},
		Uuid: tasks.Id,
	}
	return &message

}

func (d *AnsibleDispatcher) RunnerOnSkipped(
	runnerID string,
	playbookFilename string,
	playbookUUID string,
	msgCounter int,
	host *string,
	tasks *results.AnsiblePlaybookJSONResultsPlayTaskItem,
	hosts *results.AnsiblePlaybookJSONResultsPlayTaskHostsItem) *message.AnsibleRunnerJobEventYaml {
	message := message.AnsibleRunnerJobEventYaml{
		Event:       "v2_runner_on_skipped",
		RunnerIdent: &runnerID,
		// StartLine:   0, //Not available
		// EndLine:     0, //Not available
		Created: &tasks.Duration.Start,
		Counter: msgCounter,
		Stdout:  hosts.Stdout,
		Stderr:  hosts.Stderr,
		EventData: &message.AnsibleRunnerJobEventYamlEventData{
			Playbook:     &playbookFilename,
			PlaybookUuid: &playbookUUID,
			Host:         host,
			TaskUuid:     &tasks.Id,
			Task:         &tasks.Name,
			TaskAction:   &hosts.Action,
			Res: &message.AnsibleRunnerJobEventYamlEventDataRes{
				Changed: &hosts.Changed,
			},
			Uuid: &tasks.Id,
		},
		Uuid: tasks.Id,
	}
	return &message
}

// https://github.com/ansible/ansible-runner/blob/7e60c1f61ee6118f0b943e7897dc3a58ecc043bd/ansible_runner/display_callback/callback/awx_display.py#L530
func (d *AnsibleDispatcher) PlaybookOnTaskStart(
	runnerID string,
	playbookFilename string,
	playbookUUID string,
	msgCounter int,
	host *string,
	tasks *results.AnsiblePlaybookJSONResultsPlayTaskItem,
	hosts *results.AnsiblePlaybookJSONResultsPlayTaskHostsItem,
	taskUUIDs map[string]int,
	duplicateTaskCount map[string]int) *message.AnsibleRunnerJobEventYaml {
	taskUUID := tasks.Id
	if occurence, found := taskUUIDs[taskUUID]; found {
		if occurence > 1 {
			// 	 When this task UUID repeats, it means the play is using the
			//  free strategy (or serial:1) so different hosts may be running
			//  different tasks within a play (where duplicate UUIDS are common).
			//
			//  When this is the case, modify the UUID slightly to append
			//  a counter so we can still _track_ duplicate events, but also
			//  avoid breaking the display in these scenarios.
			duplicateTaskCount[taskUUID]++
			taskUUID = taskUUID + "_" + strconv.Itoa(duplicateTaskCount[taskUUID])
			tasks.Id = taskUUID

		}
	}
	message := message.AnsibleRunnerJobEventYaml{
		Event:       "playbook_on_task_start",
		RunnerIdent: &runnerID,
		// StartLine:   0, //Not available
		// EndLine:     0, //Not available
		Created: &tasks.Duration.Start,
		Counter: msgCounter,
		Stdout:  hosts.Stdout,
		EventData: &message.AnsibleRunnerJobEventYamlEventData{
			Playbook:     &playbookFilename,
			PlaybookUuid: &playbookUUID,
			Host:         host,
			TaskUuid:     &taskUUID,
			Task:         &tasks.Name,
			TaskAction:   &hosts.Action,
			Uuid:         &taskUUID,
		},
		Uuid: taskUUID,
	}
	return &message
}

// https://github.com/ansible/ansible-runner/blob/7e60c1f61ee6118f0b943e7897dc3a58ecc043bd/ansible_runner/display_callback/callback/awx_display.py#L477
func (d *AnsibleDispatcher) PlaybookOnPlayStart(playbookFilename string, msgCounter int, play *results.AnsiblePlaybookJSONResultsPlay) *message.AnsibleRunnerJobEventYaml {
	jobEvent := message.AnsibleRunnerJobEventYaml{
		Event:       "playbook_on_play_start",
		RunnerIdent: &d.deviceID,
		Counter:     msgCounter,
		Created:     &play.Play.Duration.Start,
		EventData: &message.AnsibleRunnerJobEventYamlEventData{
			PlaybookUuid: &play.Play.Id,
			Playbook:     &playbookFilename,
			Host:         getHostsString(play.Tasks),
			Uuid:         &play.Play.Id,
		},
		Uuid: play.Play.Id,
	}
	return &jobEvent
}

// https://github.com/ansible/ansible-runner/blob/7e60c1f61ee6118f0b943e7897dc3a58ecc043bd/ansible_runner/display_callback/callback/awx_display.py#L442
func (d *AnsibleDispatcher) PlaybookOnStart(playbookFilename string, msgCounter int, event *results.AnsiblePlaybookJSONResultsPlay) *message.AnsibleRunnerJobEventYaml {
	uuid := uuid.New().String()
	message := message.AnsibleRunnerJobEventYaml{
		Event:       "playbook_on_start",
		Created:     &event.Play.Duration.Start,
		RunnerIdent: &d.deviceID,
		Counter:     msgCounter,
		EventData: &message.AnsibleRunnerJobEventYamlEventData{
			PlaybookUuid: &uuid,
			Playbook:     &playbookFilename,
			Host:         getHostsString(event.Tasks),
			Uuid:         &event.Play.Id,
		},
		Uuid: uuid,
	}
	return &message
}

func getHostsString(event []results.AnsiblePlaybookJSONResultsPlayTask) *string {
	hostsString := ""
	for _, task := range event {
		hosts := make([]string, 0, len(task.Hosts))

		for k := range task.Hosts {
			hosts = append(hosts, k)
		}
		hostsString = strings.Join(hosts, ",")
	}
	return &hostsString
}

func countOccurence(tasks []results.AnsiblePlaybookJSONResultsPlayTask) map[string]int {
	dict := make(map[string]int)
	for _, task := range tasks {
		dict[task.Task.Id]++
	}
	return dict
}

// required event for cloud connector
func (d *AnsibleDispatcher) ExecutorOnStart(correlationID string, stdout string) message.AnsibleRunnerJobEventYaml {
	return message.AnsibleRunnerJobEventYaml{
		Event:     "executor_on_start",
		Uuid:      uuid.New().String(),
		Counter:   -1,
		Stdout:    stdout,
		StartLine: 0,
		EndLine:   0,
		EventData: &message.AnsibleRunnerJobEventYamlEventData{
			CrcDispatcherCorrelationId: &correlationID,
		},
	}
}

// required event for cloud connector
func (d *AnsibleDispatcher) ExecutorOnFailed(correlationID string, stdout string, errorCode string, errorDetails string) message.AnsibleRunnerJobEventYaml {
	return message.AnsibleRunnerJobEventYaml{
		Event:     "executor_on_failed",
		Uuid:      uuid.New().String(),
		Counter:   -1,
		Stdout:    stdout,
		StartLine: 0,
		EndLine:   0,
		EventData: &message.AnsibleRunnerJobEventYamlEventData{
			CrcDispatcherCorrelationId: &correlationID,
			CrcDispatcherErrorCode:     &errorCode,
			CrcDispatcherErrorDetails:  &errorDetails,
		},
	}
}

// Create the message with event data to send back to Dispatcher
func ComposeDispatcherMessage(events []message.AnsibleRunnerJobEventYaml, returnURL string, responseTo string) (*pb.Data, error) {

	content, err := CreateMessage(events)
	if err != nil {
		return nil, err
	}

	return &pb.Data{
		MessageId:  uuid.New().String(),
		Directive:  returnURL,
		ResponseTo: responseTo,
		Metadata:   map[string]string{},
		Content:    content,
	}, nil
}

func CreateMessage(events []message.AnsibleRunnerJobEventYaml) ([]byte, error) {
	buf := new(bytes.Buffer)

	for idx, event := range events {
		b, err := json.Marshal(event)
		if err != nil {
			return nil, err
		}
		buf.Write(b)
		if idx != len(events)-1 {
			buf.WriteString("\n")
		}
	}
	return buf.Bytes(), nil
}
