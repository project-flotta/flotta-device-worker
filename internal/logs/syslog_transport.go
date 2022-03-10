package logs

import (
	"errors"
	"fmt"
	"log/syslog"
	"net"
	"os"
	"sync"
	"time"
)

const (
	priority           = syslog.LOG_INFO
	elapsedTimeOnError = 100 * time.Millisecond
)

// We implement this way to force timestamp with the original ones.
type syslogTransport struct {
	server   string
	protocol string
	hostname string
	conn     net.Conn
	lock     sync.Mutex // guards conn
}

func NewSyslogServer(server string, protocol string, deviceID string) (*syslogTransport, error) {
	proto := "tcp"
	if protocol != "" {
		proto = protocol
	}
	syslogInstance := &syslogTransport{
		server:   server,
		protocol: proto,
		hostname: deviceID,
		lock:     sync.Mutex{},
	}
	err := syslogInstance.Init()
	if err != nil {
		return nil, err
	}
	return syslogInstance, nil
}

func (s *syslogTransport) Init() error {
	var err error
	s.lock.Lock()
	defer s.lock.Unlock()

	s.conn, err = net.Dial(s.protocol, s.server)
	if err != nil {
		return err
	}
	return err
}

func (s *syslogTransport) WriteLog(logline *LogEntry) error {
	s.lock.Lock()
	hostname := s.hostname
	s.lock.Unlock()

	logEntry := fmt.Sprintf("<%d>%s %s %s[%d]: %s\n",
		priority, logline.Time.Format(time.RFC3339), hostname,
		logline.workload, os.Getpid(), logline.String())
	return s.writeAndRetry(logEntry, 10)
}

func (s *syslogTransport) writeAndRetry(data string, retryTimes int) error {
	if retryTimes < 0 {
		return errors.New("cannot send message")
	}

	s.lock.Lock()
	if s.conn != nil {
		_, err := fmt.Fprintf(s.conn, "%s", data)
		s.lock.Unlock()
		if err != nil {
			time.Sleep(elapsedTimeOnError)
			return s.writeAndRetry(data, retryTimes-1)
		}
		return nil
	}
	s.lock.Unlock()
	return s.writeAndRetry(data, retryTimes-1)
}
