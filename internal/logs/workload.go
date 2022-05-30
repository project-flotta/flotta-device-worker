package logs

import (
	"context"
	"sync"
	"time"

	"git.sr.ht/~spc/go-log"
	"github.com/project-flotta/flotta-device-worker/internal/workload"
	"github.com/project-flotta/flotta-device-worker/internal/workload/podman"
	"github.com/project-flotta/flotta-operator/models"
)

type transportLog struct {
	transport transport
	log       *FIFOLog
	cancel    context.CancelFunc
}

func (t *transportLog) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	t.cancel = cancel
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				entry, err := t.log.ReadLine()
				if err != nil {
					log.Error("error on read log info", err)
				}
				if entry == nil {
					time.Sleep(elapsedTime)
				} else {
					err = t.transport.WriteLog(entry)
					if err != nil {
						// cannot send, pushed again to the queue.
						err := t.log.Write(entry)
						if err != nil {
							log.Error("cannot return log entry to the buffer log after transport failed", err)
						}
					}
				}
			}
		}
	}()
}

func (t *transportLog) Stop() {
	if t.cancel != nil {
		t.cancel()
	}
}

type WorkloadsLogsTarget struct {
	deviceID            string
	transports          map[string]transportLog
	wwriter             map[string]*WorkloadWriter // key is the workload name
	workloadsConfig     map[string]string
	workloadManager     *workload.WorkloadManager
	workloadsConfigLock sync.RWMutex
}

func NewWorkloadsLogsTarget(manager *workload.WorkloadManager) *WorkloadsLogsTarget {
	return &WorkloadsLogsTarget{
		transports:      map[string]transportLog{},
		wwriter:         map[string]*WorkloadWriter{},
		workloadsConfig: map[string]string{},
		workloadManager: manager,
	}
}

func (w *WorkloadsLogsTarget) GetCurrentTransports() []string {
	res := []string{}
	for key := range w.transports {
		res = append(res, key)
	}
	return res
}

func (w *WorkloadsLogsTarget) GetCurrentWorkloads() []*WorkloadWriter {
	res := []*WorkloadWriter{}
	for _, val := range w.wwriter {
		res = append(res, val)
	}
	return res
}

func (w *WorkloadsLogsTarget) deleteTargetTransport(name string) {
	delete(w.transports, name)
	for key, writer := range w.wwriter {
		if writer.targetName != name {
			continue
		}
		writer.Stop()
		delete(w.wwriter, key)
		log.Debugf("Stoping and removing log target transport '%s'", name)

	}
}

func (w *WorkloadsLogsTarget) deleteWorkload(name string) {
	workloadWriter, ok := w.wwriter[name]
	if ok {
		workloadWriter.Stop()
		delete(w.wwriter, name)
	}
}

func (w *WorkloadsLogsTarget) Init(config models.DeviceConfigurationMessage) error {
	w.deviceID = config.DeviceID

	err := w.Update(config)
	if err != nil {
		return err
	}

	workloads, err := w.workloadManager.ListWorkloads()
	if err != nil {
		log.Error("Logs: cannot get current list of workloads:", err)
		return err
	}

	for _, wrk := range workloads {
		w.WorkloadStarted(wrk.Name, nil)
	}

	return nil
}

func (w *WorkloadsLogsTarget) Update(config models.DeviceConfigurationMessage) error {
	original_transports := w.GetCurrentTransports()
	transports := map[string]transportLog{}

	for key, val := range config.Configuration.LogCollection {
		kind := val.Kind
		switch kind {
		case SyslogTransport:
			transport, err := NewSyslogServer(val.SyslogConfig.Address, val.SyslogConfig.Protocol, w.deviceID)
			if err != nil {
				log.Errorf("Cannot add log transport to target: %s", err)
				continue
			}
			tlog := transportLog{
				transport: transport,
				log:       NewFIFOLog(int(val.BufferSize * 1024)),
			}
			tlog.Start()
			transports[key] = tlog
			log.Debugf("Started log transport '%s'", key)
		default:
			log.Errorf("Cannot initialize log transport %s", kind)
		}
	}
	// Delete leftovers trasnports if no longer exists
	for i := range original_transports {
		key := original_transports[i]
		if _, ok := transports[key]; !ok {
			w.deleteTargetTransport(key)
		}
	}
	log.Debugf("Update log transports, number of tranports: %d", len(transports))
	w.transports = transports

	workloadsConfig := map[string]string{}
	for _, workload := range config.Workloads {
		if workload.LogCollection != "" {
			workloadsConfig[workload.Name] = workload.LogCollection
		}
	}
	w.workloadsConfigLock.Lock()
	defer w.workloadsConfigLock.Unlock()
	w.workloadsConfig = workloadsConfig
	return nil
}

func (w *WorkloadsLogsTarget) WorkloadRemoved(workloadName string) {
	w.deleteWorkload(workloadName)
}

func (w *WorkloadsLogsTarget) WorkloadStarted(workloadName string, report []*podman.PodReport) {
	// If target is present, just delete it because maybe is there any leftover
	// and want to delete in case a restart.
	w.deleteWorkload(workloadName)

	w.workloadsConfigLock.RLock()
	targetTransport, ok := w.workloadsConfig[workloadName]
	w.workloadsConfigLock.RUnlock()
	if !ok {
		return
	}

	transport, ok := w.transports[targetTransport]
	if !ok {
		log.Errorf("Log transport '%s' cannot be found in this device", targetTransport)
		return
	}

	writer := NewWorkloadWriter(workloadName, w.workloadManager)
	writer.SetTarget(targetTransport, transport.log)
	w.wwriter[workloadName] = writer
	err := writer.Start()
	if err != nil {
		log.Error("Cannot start workload logs", err)
	}
}
