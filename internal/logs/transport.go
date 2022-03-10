package logs

const (
	SyslogTransport = "syslog"
)

type transport interface {
	WriteLog(logline *LogEntry) error
}
