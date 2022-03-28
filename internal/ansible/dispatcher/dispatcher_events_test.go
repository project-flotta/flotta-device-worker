package dispatcher_test

import (
	"strings"

	"github.com/apenella/go-ansible/pkg/stdoutcallback/results"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	dispatcher "github.com/project-flotta/flotta-device-worker/internal/ansible/dispatcher"
	"github.com/project-flotta/flotta-device-worker/internal/ansible/model/message"
)

var _ = Describe("Dispatcher Events test", func() {
	var (
		ansibleDispatcher *dispatcher.AnsibleDispatcher

		deviceId           = "test-device-id-12345"
		playbookFilename   = "test-playbook-filename.yml"
		playbookUuid       = "test-plabook-uuid-0001"
		host               = "host1"
		playbookJSONResult = results.AnsiblePlaybookJSONResultsPlay{
			Play: &results.AnsiblePlaybookJSONResultsPlaysPlay{
				Name: "task-test-name-1",
				Id:   "9k7b6825-cf3d-4653-9010-9f75d9358c54",
				Duration: &results.AnsiblePlaybookJSONResultsPlayDuration{
					Start: "2022-02-03T08:46:38.407474Z",
					End:   "2022-02-03T08:48:38.407474Z",
				},
			},
			Tasks: []results.AnsiblePlaybookJSONResultsPlayTask{
				{
					Task: &results.AnsiblePlaybookJSONResultsPlayTaskItem{
						Name: "task-test-name-1",
						Id:   "9k7b6825-cf3d-4653-9010-9f75d9358c54",
						Duration: &results.AnsiblePlaybookJSONResultsPlayTaskItemDuration{
							Start: "2022-02-03T08:46:38.412345Z",
							End:   "2022-02-03T08:46:38.412358Z",
						},
					},
					Hosts: map[string]*results.AnsiblePlaybookJSONResultsPlayTaskHostsItem{
						"host1": {
							Action:       "",
							Changed:      true,
							Msg:          "this is a message",
							AnsibleFacts: map[string]interface{}{"factkey1": "factvalue1"},
							Stdout:       "",
							StdoutLines:  []string{"line1, line2"},
							Stderr:       "",
							StderrLines:  []string{},
						},
					},
				},
			},
		}
	)
	BeforeEach(func() {
		ansibleDispatcher = dispatcher.NewAnsibleDispatcher("test-device-id-123457")
	})
	Context("HTTP Request to ingress service", func() {
		It("Compose Dispatcher Message", func() {
			//Given
			responseTo := "00000001"
			testEvent := []message.AnsibleRunnerJobEventYaml{
				{
					Event:     "test-event",
					Uuid:      "uuid-1234",
					Counter:   -1,
					Stdout:    "",
					StartLine: 0,
					EndLine:   0,
					EventData: &message.AnsibleRunnerJobEventYamlEventData{},
				},
			}
			//When
			data, err := dispatcher.ComposeDispatcherMessage(testEvent, "test_return_url", responseTo)
			//Expect
			Expect(err).ShouldNot(HaveOccurred())
			Expect(data.Metadata).To(BeEmpty())
		})

	})
	Context("Compose JSON Message", func() {
		It("Add event", func() {
			//Given
			testEvent := message.AnsibleRunnerJobEventYaml{
				Event:     "test-event",
				Uuid:      "uuid-1234",
				Counter:   -1,
				Stdout:    "",
				StartLine: 0,
				EndLine:   0,
				EventData: &message.AnsibleRunnerJobEventYamlEventData{},
			}

			//When
			ansibleDispatcher.AddRunnerJobEvent(testEvent)
			//Expect
			Expect(len(ansibleDispatcher.GetMsgList())).To(Equal(1))
		})
		It("Playbook on start", func() {
			//Given
			crcDispatcherCorrelationId := "d4ae95cf-71fd-4386-8dbf-2bce933ce713"
			expectedJson :=
				`{  "counter":-1,
					"end_line":0,
					"event":"executor_on_start",
					"event_data":{"crc_dispatcher_correlation_id":"d4ae95cf-71fd-4386-8dbf-2bce933ce713"},
					"start_line":0,
					"stdout":"",
					"uuid":"REPLACE_UUID"}`

			msgExecutorOnStart := ansibleDispatcher.ExecutorOnStart(crcDispatcherCorrelationId, "")

			expectedJson = replaceIDs(expectedJson, msgExecutorOnStart.Uuid, "")

			msgList := ansibleDispatcher.AddRunnerJobEvent(msgExecutorOnStart)

			//When
			jsonBytes, err := dispatcher.CreateMessage(msgList)

			//Expect
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(jsonBytes)).To(Equal(expectedJson))
		})
		It("Playbook on play start", func() {
			//Given
			// crcDispatcherCorrelationId := "d4ae95cf-71fd-4386-8dbf-2bce933ce713"
			expectedJson :=
				`{  "counter":1,
					"created":"2022-02-03T08:46:38.407474Z",
					"end_line":0,
					"event":"playbook_on_start",
					"event_data":{
						"host":"host1",
						"playbook":"test-playbook-filename.yml",
						"playbook_uuid":"REPLACE_PLAYBOOK_UUID",
						"uuid": "9k7b6825-cf3d-4653-9010-9f75d9358c54"
					},
					"runner_ident":"test-device-id-123457",
					"start_line":0,
					"uuid":"REPLACE_UUID"}`

			msgPlaybookOnStart := ansibleDispatcher.PlaybookOnStart(playbookFilename, 1, &playbookJSONResult)

			expectedJson = replaceIDs(expectedJson, msgPlaybookOnStart.Uuid, *msgPlaybookOnStart.EventData.PlaybookUuid)

			msgList := ansibleDispatcher.AddRunnerJobEvent(*msgPlaybookOnStart)

			//When
			jsonBytes, err := dispatcher.CreateMessage(msgList)

			//Expect
			Expect(err).ShouldNot(HaveOccurred())
			Expect(string(jsonBytes)).ShouldNot(BeNil())
			Expect(string(jsonBytes)).To(Equal(expectedJson))
		})

	})
	It("Playbook on play start", func() {
		//Given
		// crcDispatcherCorrelationId := "d4ae95cf-71fd-4386-8dbf-2bce933ce713"
		expectedJson :=
			`{  "counter":1,
				"created":"2022-02-03T08:46:38.407474Z",
				"end_line":0,
				"event":"playbook_on_play_start",
				"event_data":{
					"host":"host1",
					"playbook":"test-playbook-filename.yml",
					"playbook_uuid":"REPLACE_PLAYBOOK_UUID",
					"uuid": "9k7b6825-cf3d-4653-9010-9f75d9358c54"
				},
				"runner_ident":"test-device-id-123457",
				"start_line":0,
				"uuid":"REPLACE_UUID"}`

		msgPlaybookOnPlayStart := ansibleDispatcher.PlaybookOnPlayStart(playbookFilename, 1, &playbookJSONResult)

		expectedJson = replaceIDs(expectedJson, msgPlaybookOnPlayStart.Uuid, *msgPlaybookOnPlayStart.EventData.PlaybookUuid)

		msgList := ansibleDispatcher.AddRunnerJobEvent(*msgPlaybookOnPlayStart)

		//When
		jsonBytes, err := dispatcher.CreateMessage(msgList)
		//Expect
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(jsonBytes)).To(Equal(expectedJson))
	})
	It("Playbook on task start", func() {
		//Given
		expectedJson :=
			`{ "counter":3,
				"created":"2022-02-03T08:46:38.412345Z",
				"end_line":0,
				"event":"playbook_on_task_start",
				"event_data":{
					"host":"host1",
					"playbook":"test-playbook-filename.yml",
					"playbook_uuid":"REPLACE_PLAYBOOK_UUID",
					"task":"task-test-name-1",
					"task_action":"",
					"task_uuid":"9k7b6825-cf3d-4653-9010-9f75d9358c54",
					"uuid": "9k7b6825-cf3d-4653-9010-9f75d9358c54"
				},
				"runner_ident":"test-device-id-12345",
				"start_line":0,
				"stdout":"",
				"uuid":"REPLACE_UUID"}`

		taskUUIDs := make(map[string]int)
		duplicates := make(map[string]int)
		msgPlaybookOnStart := ansibleDispatcher.PlaybookOnTaskStart(
			deviceId,
			playbookFilename,
			playbookUuid,
			3,
			&host,
			playbookJSONResult.Tasks[0].Task,
			playbookJSONResult.Tasks[0].Hosts[host],
			taskUUIDs,
			duplicates)

		expectedJson = replaceIDs(expectedJson, msgPlaybookOnStart.Uuid, *msgPlaybookOnStart.EventData.PlaybookUuid)

		msgList := ansibleDispatcher.AddRunnerJobEvent(*msgPlaybookOnStart)

		//When
		jsonBytes, err := dispatcher.CreateMessage(msgList)

		//Expect
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(jsonBytes)).To(Equal(expectedJson))
	})
	It("Playbook on runner ok", func() {
		//Given
		expectedJson :=
			`{	"counter":4,
				"created":"2022-02-03T08:46:38.412345Z",
				"end_line":0,
				"event":"v2_runner_on_ok",
				"event_data":{
					"host":"host1",
					"playbook":"test-playbook-filename.yml",
					"playbook_uuid":"REPLACE_PLAYBOOK_UUID",
					"res":{"changed":true},
					"task":"task-test-name-1",
					"task_action":"",
					"task_uuid":"9k7b6825-cf3d-4653-9010-9f75d9358c54",
					"uuid": "9k7b6825-cf3d-4653-9010-9f75d9358c54"
				},
				"runner_ident":"test-device-id-12345",
				"start_line":0,
				"stdout":"",
				"uuid":"REPLACE_UUID"}`

		msgRunnerOk := ansibleDispatcher.RunnerOnOk(
			deviceId,
			playbookFilename,
			playbookUuid,
			4,
			&host,
			playbookJSONResult.Tasks[0].Task,
			playbookJSONResult.Tasks[0].Hosts[host])

		expectedJson = replaceIDs(expectedJson, msgRunnerOk.Uuid, *msgRunnerOk.EventData.PlaybookUuid)

		msgList := []message.AnsibleRunnerJobEventYaml{*msgRunnerOk}

		//When
		jsonBytes, err := dispatcher.CreateMessage(msgList)

		//Expect
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(jsonBytes)).To(Equal(expectedJson))
	})
	It("Playbook on runner failed", func() {
		//Given
		expectedJson :=
			`{	"counter":4,
				"created":"2022-02-03T08:46:38.412345Z",
				"end_line":0,
				"event":"v2_runner_on_failed",
				"event_data":{
					"host":"host1",
					"playbook":"test-playbook-filename.yml",
					"playbook_uuid":"REPLACE_PLAYBOOK_UUID",
					"res":{"changed":true},
					"task":"task-test-name-1",
					"task_action":"",
					"task_uuid":"9k7b6825-cf3d-4653-9010-9f75d9358c54",
					"uuid": "9k7b6825-cf3d-4653-9010-9f75d9358c54"
				},
				"runner_ident":"test-device-id-12345",
				"start_line":0,
				"stderr":"",
				"stdout":"",
				"uuid":"REPLACE_UUID"}`

		msgRunnerFailed := ansibleDispatcher.RunnerOnFailed(
			deviceId,
			playbookFilename,
			playbookUuid,
			4,
			&host,
			playbookJSONResult.Tasks[0].Task,
			playbookJSONResult.Tasks[0].Hosts[host])

		expectedJson = replaceIDs(expectedJson, msgRunnerFailed.Uuid, *msgRunnerFailed.EventData.PlaybookUuid)

		msgList := []message.AnsibleRunnerJobEventYaml{*msgRunnerFailed}

		//When
		jsonBytes, err := dispatcher.CreateMessage(msgList)

		//Expect
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(jsonBytes)).To(Equal(expectedJson))
	})
	It("Playbook on runner skipped", func() {
		//Given
		expectedJson :=
			`{  "counter":4,
				"created":"2022-02-03T08:46:38.412345Z",
				"end_line":0,
				"event":"v2_runner_on_skipped",
				"event_data":{
					"host":"host1",
					"playbook":"test-playbook-filename.yml",
					"playbook_uuid":"REPLACE_PLAYBOOK_UUID",
					"res":{"changed":true},
					"task":"task-test-name-1",
					"task_action":"",
					"task_uuid":"9k7b6825-cf3d-4653-9010-9f75d9358c54",
					"uuid": "9k7b6825-cf3d-4653-9010-9f75d9358c54"
				},
				"runner_ident":"test-device-id-12345",
				"start_line":0,
				"stderr":"",
				"stdout":"",
				"uuid":"REPLACE_UUID"}`

		msgRunnerSkipped := ansibleDispatcher.RunnerOnSkipped(
			deviceId,
			playbookFilename,
			playbookUuid,
			4,
			&host,
			playbookJSONResult.Tasks[0].Task,
			playbookJSONResult.Tasks[0].Hosts[host])

		expectedJson = replaceIDs(expectedJson, msgRunnerSkipped.Uuid, *msgRunnerSkipped.EventData.PlaybookUuid)
		msgList := []message.AnsibleRunnerJobEventYaml{*msgRunnerSkipped}

		//When
		jsonBytes, err := dispatcher.CreateMessage(msgList)

		//Expect
		Expect(err).ShouldNot(HaveOccurred())
		Expect(string(jsonBytes)).To(Equal(expectedJson))
	})
})

func replaceIDs(expectedJson string, uuid string, playbookUuid string) string {
	if uuid != "" {
		expectedJson = strings.Replace(expectedJson, "REPLACE_UUID", uuid, -1)
	}
	if playbookUuid != "" {
		expectedJson = strings.Replace(expectedJson, "REPLACE_PLAYBOOK_UUID", playbookUuid, 1)
	}
	expectedJson = strings.ReplaceAll(expectedJson, "\t", "")
	expectedJson = strings.ReplaceAll(expectedJson, "\n", "")
	expectedJson = strings.ReplaceAll(expectedJson, " ", "")
	return expectedJson

}
