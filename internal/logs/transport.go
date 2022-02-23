package logs

type transport interface {
	WriteLog(logline *LogEntry, workload string) error
}
