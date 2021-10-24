package driver

import (
	"context"
	"fmt"
	"sync"
	"syscall"

	"github.com/containerd/fifo"
	"github.com/docker/docker/daemon/logger"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type FluentDriver struct {
	mu      sync.Mutex
	loggers map[string]*fluentLogger
}

func New() *FluentDriver {
	return &FluentDriver{
		loggers: make(map[string]*fluentLogger),
	}
}

type StartLoggingRequest struct {
	File string
	Info logger.Info
}

func (d *FluentDriver) StartLogging(req StartLoggingRequest) error {
	if req.Info.ContainerID == "" {
		return errors.New("must provide container id in log context")
	}
	if req.File == "" {
		return errors.New("must provide path to fifo stream in log context")
	}

	d.mu.Lock()
	if _, exists := d.loggers[req.File]; exists {
		d.mu.Unlock()
		return fmt.Errorf("logger for %q already exists", req.File)
	}
	d.mu.Unlock()

	logrus.WithFields(logrus.Fields{
		"id":   req.Info.ContainerID,
		"file": req.File,
	}).Debug("Start logging.")

	f, err := fifo.OpenFifo(context.Background(), req.File, syscall.O_RDONLY, 0700)
	if err != nil {
		return errors.Wrapf(err, "error opening logger fifo: %q", req.File)
	}

	l, err := newLogger(req.Info, f)
	if err != nil {
		return errors.Wrap(err, "error creating fluent logger")
	}

	d.mu.Lock()
	d.loggers[req.File] = l
	d.mu.Unlock()

	go l.consumeLogs()

	return nil
}

type StopLoggingRequest struct {
	File string
}

func (d *FluentDriver) StopLogging(req StopLoggingRequest) error {
	logrus.WithFields(logrus.Fields{
		"file": req.File,
	}).Debug("Stop logging.")

	d.mu.Lock()
	if l, ok := d.loggers[req.File]; ok {
		l.Close()
		delete(d.loggers, req.File)
	}
	d.mu.Unlock()

	return nil
}
