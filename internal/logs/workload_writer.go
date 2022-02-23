package logs

import (
	"context"
	"fmt"

	"github.com/project-flotta/flotta-device-worker/internal/workload"
)

type WorkloadWriter struct {
	workloadName string
	cancel       context.CancelFunc
	target       *FIFOLog
	targetName   string
	manager      *workload.WorkloadManager
}

func NewWorkloadWriter(workload string, manager *workload.WorkloadManager) *WorkloadWriter {
	res := &WorkloadWriter{
		workloadName: workload,
		manager:      manager,
	}
	return res
}

func (ww *WorkloadWriter) SetTarget(name string, target *FIFOLog) {
	ww.target = target
	ww.targetName = name
}

func (ww *WorkloadWriter) Write(data []byte) (n int, err error) {
	entry := NewLogEntry(data, ww.workloadName)
	if ww.target == nil {
		return 0, fmt.Errorf("cannot write to target, no buffer is defined")
	}
	err = ww.target.Write(entry)
	if err != nil {
		return 0, fmt.Errorf("cannot write to target: %v", err)
	}
	return len(data), nil
}

// Stop writer.
func (ww *WorkloadWriter) Stop() {
	if ww.cancel == nil {
		return
	}
	ww.cancel()
}

// Start writer.
func (ww *WorkloadWriter) Start() error {
	cancel, err := ww.manager.Logs(ww.workloadName, ww)
	ww.cancel = cancel
	if err != nil {
		return err
	}

	return nil
}
