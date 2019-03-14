package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"k8s.io/klog"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/util/tail"

	corev1 "k8s.io/api/core/v1"
)

const (
	// timeFormat is the time format used in the log.
	timeFormat = time.RFC3339Nano
	// blockSize is the block size used in tail.
	blockSize = 1024

	// stateCheckPeriod is the period to check container state while following
	// the container log. Kubelet should not keep following the log when the
	// container is not running.
	stateCheckPeriod = 5 * time.Second
)

var (
	// eol is the end-of-line sign in the log.
	eol = []byte{'\n'}
	// delimiter is the delimiter for timestamp and stream type in log line.
	delimiter = []byte{' '}
	// tagDelimiter is the delimiter for log tags.
	tagDelimiter = []byte(runtimeapi.LogTagDelimiter)
)

// logMessage is the CRI internal log type.
type logMessage struct {
	timestamp time.Time
	stream    runtimeapi.LogStreamType
	log       []byte
}

// reset resets the log to nil.
func (l *logMessage) reset() {
	l.timestamp = time.Time{}
	l.stream = ""
	l.log = nil
}

// LogOptions is the CRI internal type of all log options.
type LogOptions struct {
	tail      int64
	bytes     int64
	since     time.Time
	follow    bool
	timestamp bool
}

// NewLogOptions convert the v1.PodLogOptions to CRI internal LogOptions.
func NewLogOptions(apiOpts *corev1.PodLogOptions, now time.Time) *LogOptions {
	opts := &LogOptions{
		tail:      -1, // -1 by default which means read all logs.
		bytes:     -1, // -1 by default which means read all logs.
		follow:    apiOpts.Follow,
		timestamp: apiOpts.Timestamps,
	}
	if apiOpts.TailLines != nil {
		opts.tail = *apiOpts.TailLines
	}
	if apiOpts.LimitBytes != nil {
		opts.bytes = *apiOpts.LimitBytes
	}
	if apiOpts.SinceSeconds != nil {
		opts.since = now.Add(-time.Duration(*apiOpts.SinceSeconds) * time.Second)
	}
	if apiOpts.SinceTime != nil && apiOpts.SinceTime.After(opts.since) {
		opts.since = apiOpts.SinceTime.Time
	}
	return opts
}

// parseFunc is a function parsing one log line to the internal log type.
// Notice that the caller must make sure logMessage is not nil.
type parseFunc func([]byte, *logMessage) error

var parseFuncs = []parseFunc{
	parseCRILog, // CRI log format parse function
}

// parseCRILog parses logs in CRI log format. CRI Log format example:
//   2016-10-06T00:17:09.669794202Z stdout P log content 1
//   2016-10-06T00:17:09.669794203Z stderr F log content 2
func parseCRILog(log []byte, msg *logMessage) error {
	var err error
	// Parse timestamp
	idx := bytes.Index(log, delimiter)
	if idx < 0 {
		return fmt.Errorf("timestamp is not found")
	}
	msg.timestamp, err = time.Parse(timeFormat, string(log[:idx]))
	if err != nil {
		return fmt.Errorf("unexpected timestamp format %q: %v", timeFormat, err)
	}

	// Parse stream type
	log = log[idx+1:]
	idx = bytes.Index(log, delimiter)
	if idx < 0 {
		return fmt.Errorf("stream type is not found")
	}
	msg.stream = runtimeapi.LogStreamType(log[:idx])
	if msg.stream != runtimeapi.Stdout && msg.stream != runtimeapi.Stderr {
		return fmt.Errorf("unexpected stream type %q", msg.stream)
	}

	// Parse log tag
	log = log[idx+1:]
	idx = bytes.Index(log, delimiter)
	if idx < 0 {
		return fmt.Errorf("log tag is not found")
	}
	// Keep this forward compatible.
	tags := bytes.Split(log[:idx], tagDelimiter)
	partial := (runtimeapi.LogTag(tags[0]) == runtimeapi.LogTagPartial)
	// Trim the tailing new line if this is a partial line.
	if partial && len(log) > 0 && log[len(log)-1] == '\n' {
		log = log[:len(log)-1]
	}

	// Get log content
	msg.log = log[idx+1:]

	return nil
}

// getParseFunc returns proper parse function based on the sample log line passed in.
func getParseFunc(log []byte) (parseFunc, error) {
	for _, p := range parseFuncs {
		if err := p(log, &logMessage{}); err == nil {
			return p, nil
		}
	}
	return nil, fmt.Errorf("unsupported log format: %q", log)
}

// logWriter controls the writing into the stream based on the log options.
type logWriter struct {
	stdout io.Writer
	stderr io.Writer
	opts   *LogOptions
	remain int64
}

// errMaximumWrite is returned when all bytes have been written.
var errMaximumWrite = errors.New("maximum write")

// errShortWrite is returned when the message is not fully written.
var errShortWrite = errors.New("short write")

func newLogWriter(stdout io.Writer, stderr io.Writer, opts *LogOptions) *logWriter {
	w := &logWriter{
		stdout: stdout,
		stderr: stderr,
		opts:   opts,
		remain: math.MaxInt64, // initialize it as infinity
	}
	if opts.bytes >= 0 {
		w.remain = opts.bytes
	}
	return w
}

// writeLogs writes logs into stdout, stderr.
func (w *logWriter) write(msg *logMessage) error {
	if msg.timestamp.Before(w.opts.since) {
		// Skip the line because it's older than since
		return nil
	}
	line := msg.log
	if w.opts.timestamp {
		prefix := append([]byte(msg.timestamp.Format(timeFormat)), delimiter[0])
		line = append(prefix, line...)
	}
	// If the line is longer than the remaining bytes, cut it.
	if int64(len(line)) > w.remain {
		line = line[:w.remain]
	}
	// Get the proper stream to write to.
	var stream io.Writer
	switch msg.stream {
	case runtimeapi.Stdout:
		stream = w.stdout
	case runtimeapi.Stderr:
		stream = w.stderr
	default:
		return fmt.Errorf("unexpected stream type %q", msg.stream)
	}
	n, err := stream.Write(line)
	w.remain -= int64(n)
	if err != nil {
		return err
	}
	// If the line has not been fully written, return errShortWrite
	if n < len(line) {
		return errShortWrite
	}
	// If there are no more bytes left, return errMaximumWrite
	if w.remain <= 0 {
		return errMaximumWrite
	}
	return nil
}

type ContainerStatusFunc func(containerID string) (*runtimeapi.ContainerStatus, error)

// ReadLogs read the container log and redirect into stdout and stderr.
// Note that containerID is only needed when following the log, or else
// just pass in empty string "".
func ReadLogs(ctx context.Context, path, containerID string, opts *LogOptions, containerStatusFunc ContainerStatusFunc, stdout, stderr io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open log file %q: %v", path, err)
	}
	defer f.Close()

	// Search start point based on tail line.
	start, err := tail.FindTailLineStartIndex(f, opts.tail)
	if err != nil {
		return fmt.Errorf("failed to tail %d lines of log file %q: %v", opts.tail, path, err)
	}
	if _, err := f.Seek(start, io.SeekStart); err != nil {
		return fmt.Errorf("failed to seek %d in log file %q: %v", start, path, err)
	}

	// Start parsing the logs.
	r := bufio.NewReader(f)
	// Do not create watcher here because it is not needed if `Follow` is false.
	var watcher *fsnotify.Watcher
	var parse parseFunc
	var stop bool
	writer := newLogWriter(stdout, stderr, opts)
	msg := &logMessage{}
	for {
		if stop {
			klog.V(2).Infof("Finish parsing log file %q", path)
			return nil
		}
		l, err := r.ReadBytes(eol[0])
		if err != nil {
			if err != io.EOF { // This is an real error
				return fmt.Errorf("failed to read log file %q: %v", path, err)
			}
			if opts.follow {
				// Reset seek so that if this is an incomplete line,
				// it will be read again.
				if _, err := f.Seek(-int64(len(l)), io.SeekCurrent); err != nil {
					return fmt.Errorf("failed to reset seek in log file %q: %v", path, err)
				}
				if watcher == nil {
					// Initialize the watcher if it has not been initialized yet.
					if watcher, err = fsnotify.NewWatcher(); err != nil {
						return fmt.Errorf("failed to create fsnotify watcher: %v", err)
					}
					defer watcher.Close()
					if err := watcher.Add(f.Name()); err != nil {
						return fmt.Errorf("failed to watch file %q: %v", f.Name(), err)
					}
				}
				// Wait until the next log change.
				if found, err := waitLogs(ctx, containerID, watcher, containerStatusFunc); !found {
					return err
				}
				continue
			}
			// Should stop after writing the remaining content.
			stop = true
			if len(l) == 0 {
				continue
			}
			klog.Warningf("Incomplete line in log file %q: %q", path, l)
		}
		if parse == nil {
			// Initialize the log parsing function.
			parse, err = getParseFunc(l)
			if err != nil {
				return fmt.Errorf("failed to get parse function: %v", err)
			}
		}
		// Parse the log line.
		msg.reset()
		if err := parse(l, msg); err != nil {
			klog.Errorf("Failed with err %v when parsing log for log file %q: %q", err, path, l)
			continue
		}
		// Write the log line into the stream.
		if err := writer.write(msg); err != nil {
			if err == errMaximumWrite {
				klog.V(2).Infof("Finish parsing log file %q, hit bytes limit %d(bytes)", path, opts.bytes)
				return nil
			}
			klog.Errorf("Failed with err %v when writing log for log file %q: %+v", err, path, msg)
			return err
		}
	}
}

// waitLogs wait for the next log write. It returns a boolean and an error. The boolean
// indicates whether a new log is found; the error is error happens during waiting new logs.
func waitLogs(ctx context.Context, id string, w *fsnotify.Watcher, runtimeService ContainerStatusFunc) (bool, error) {
	// no need to wait if the pod is not running
	if running, err := isContainerRunning(id, runtimeService); !running {
		return false, err
	}
	errRetry := 5
	for {
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("context cancelled")
		case e := <-w.Events:
			switch e.Op {
			case fsnotify.Write:
				return true, nil
			default:
				klog.Errorf("Unexpected fsnotify event: %v, retrying...", e)
			}
		case err := <-w.Errors:
			klog.Errorf("Fsnotify watch error: %v, %d error retries remaining", err, errRetry)
			if errRetry == 0 {
				return false, err
			}
			errRetry--
		case <-time.After(stateCheckPeriod):
			if running, err := isContainerRunning(id, runtimeService); !running {
				return false, err
			}
		}
	}
}

func isContainerRunning(id string, r ContainerStatusFunc) (bool, error) {
	s, err := r(id)
	if err != nil {
		return false, err
	}
	// Only keep following container log when it is running.
	if s.State != runtimeapi.ContainerState_CONTAINER_RUNNING {
		klog.V(5).Infof("Container %q is not running (state=%q)", id, s.State)
		// Do not return error because it's normal that the container stops
		// during waiting.
		return false, nil
	}
	return true, nil
}
