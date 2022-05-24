package logs

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	elapsedTime = 100 * time.Millisecond
)

type LogEntry struct {
	Time     time.Time
	data     []byte
	workload string
}

func NewLogEntry(data []byte, workload string) *LogEntry {
	return &LogEntry{
		data:     data,
		Time:     time.Now(),
		workload: workload,
	}
}

func (l *LogEntry) Size() int {
	return len(l.data)
}

func (l *LogEntry) String() string {
	if l.data == nil {
		return ""
	}
	return string(l.data)
}

func (l *LogEntry) GetWorkload() string {
	return l.workload
}

type FIFOLog struct {
	maxlen      int
	data        []*LogEntry
	lock        sync.Mutex
	currentSize int
}

// NewFIFOLog returns a new FifoLog struct
func NewFIFOLog(maxSize int) *FIFOLog {
	return &FIFOLog{
		data:   []*LogEntry{},
		maxlen: maxSize,
	}
}

// CurrentSize returns the buffer size.
func (f *FIFOLog) CurrentSize() int {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.currentSize
}

// Write fills the current buffer with the next LogEntry
func (f *FIFOLog) Write(entry *LogEntry) error {
	if entry == nil {
		return errors.New("no log entry added")
	}
	f.lock.Lock()
	defer f.lock.Unlock()

	// We have here a "soft" limit, current entry size can be bigger than
	// f.maxlen but we only pass once.
	if f.currentSize > f.maxlen {
		return fmt.Errorf("buffer is already full")
	}
	f.data = append(f.data, entry)
	f.currentSize = f.currentSize + entry.Size()
	return nil
}

// ReadLine reads the line and delete it from the current buffer.
func (f *FIFOLog) ReadLine() (*LogEntry, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	if f.currentSize <= 0 {
		return nil, nil
	}
	res := f.data[0]
	f.data = f.data[1:]
	f.currentSize = f.currentSize - res.Size()
	return res, nil
}
