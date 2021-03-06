//
// Forked and simplified from http://golang.org/src/pkg/log/syslog/syslog.go
// Fork needed to set the propper hostname in the write() function
//

package sinks

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type SyslogWriter interface {
	Connect() error
	WriteStdout(b []byte, source string, timestamp int64) (int, error)
	WriteStderr(b []byte, source string, timestamp int64) (int, error)
	Close() error
	IsConnected() bool
	SetConnected(bool)
}

type writer struct {
	appId   string
	network string
	raddr   string

	connected bool

	mu   sync.Mutex // guards conn
	conn net.Conn
}

func NewSyslogWriter(network, raddr string, appId string) (w *writer) {
	return &writer{
		appId:     appId,
		network:   network,
		raddr:     raddr,
		connected: false,
	}
}

func (w *writer) Connect() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	err := w.connect()
	if err == nil {
		w.SetConnected(true)
	}
	return err
}

// connect makes a connection to the syslog server.
// It must be called with w.mu held.
func (w *writer) connect() (err error) {
	if w.conn != nil {
		// ignore err from close, it makes sense to continue anyway
		w.conn.Close()
		w.conn = nil
	}
	var c net.Conn
	c, err = net.Dial(w.network, w.raddr)
	if err == nil {
		w.conn = c
	}
	return
}

func (w *writer) WriteStdout(b []byte, source string, timestamp int64) (int, error) {
	return w.write(14, source, string(b), timestamp)
}

func (w *writer) WriteStderr(b []byte, source string, timestamp int64) (int, error) {
	return w.write(11, source, string(b), timestamp)
}

func (w *writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn != nil {
		err := w.conn.Close()
		w.conn = nil
		return err
	}
	return nil
}

func (w *writer) write(p int, source, msg string, timestamp int64) (int, error) {
	// ensure it ends in a \n
	nl := ""
	if !strings.HasSuffix(msg, "\n") {
		nl = "\n"
	}

	msg = clean(msg)
	timeString := time.Unix(0, timestamp).Format(time.RFC3339)
	timeString = strings.Replace(timeString, "Z", "+00:00", 1)

	// syslog format https://tools.ietf.org/html/rfc5424#section-6
	syslogMsg := fmt.Sprintf("<%d>1 %s %s %s %s - - %s%s", p, timeString, "loggregator", w.appId, source, msg, nl)

	// Frame msg with Octet Counting: https://tools.ietf.org/html/rfc6587#section-3.4.1
	_, err := fmt.Fprintf(w.conn, "%d %s", len(syslogMsg), syslogMsg)

	return len(msg), err
}

func (w *writer) IsConnected() bool {
	return w.connected
}

func (w *writer) SetConnected(newValue bool) {
	w.connected = newValue
}

func clean(in string) string {
	return strings.Replace(in, "\000", "", -1)
}
