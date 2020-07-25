package driver

import (
	"encoding/binary"
	"errors"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/plugins/logdriver"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/docker/daemon/logger/loggerutils"
	"github.com/docker/docker/errdefs"
	"github.com/fluent/fluent-logger-golang/fluent"
	protoio "github.com/gogo/protobuf/io"
	"github.com/sirupsen/logrus"
)

type fluentLogger struct {
	stream   io.ReadCloser
	writer   *fluent.Fluent
	metadata loggerMetadata
}

type loggerMetadata struct {
	tag           string
	containerID   string
	containerName string
	extra         map[string]string
}

func newLogger(info logger.Info, stream io.ReadCloser) (*fluentLogger, error) {
	fluentConfig, err := parseConfig(info.Config)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	tag, err := loggerutils.ParseLogTag(info, loggerutils.DefaultTemplate)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	extra, err := info.ExtraAttributes(nil)
	if err != nil {
		return nil, errdefs.InvalidParameter(err)
	}

	logrus.WithFields(logrus.Fields{
		"container": info.ContainerID,
		"config":    fluentConfig,
	}).Debug("logging driver fluentd configured")

	writer, err := fluent.New(fluentConfig)
	if err != nil {
		return nil, err
	}

	return &fluentLogger{
		stream: stream,
		writer: writer,
		metadata: loggerMetadata{
			tag:           tag,
			containerID:   info.ContainerID,
			containerName: info.ContainerName,
			extra:         extra,
		},
	}, nil
}

func (l *fluentLogger) consumeLogs() {
	dec := protoio.NewUint32DelimitedReader(l.stream, binary.BigEndian, 1e6)
	defer dec.Close()
	var buf logdriver.LogEntry

	for {
		if err := dec.ReadMsg(&buf); err != nil {
			if err == io.EOF || errors.Is(err, os.ErrClosed) {
				logrus.WithFields(logrus.Fields{
					"container": l.metadata.containerID,
				}).Debug("shutting down fluent logger")
				return
			}

			logrus.WithFields(logrus.Fields{
				"container": l.metadata.containerID,
				"error":     err.Error(),
			}).WithError(err).Error("error reading log message")

			dec = protoio.NewUint32DelimitedReader(l.stream, binary.BigEndian, 1e6)
			continue
		}

		var msg logger.Message
		msg.Line = buf.Line
		msg.Source = buf.Source
		if buf.PartialLogMetadata != nil {
			msg.PLogMetaData = &backend.PartialLogMetaData{
				ID:      buf.PartialLogMetadata.Id,
				Last:    buf.PartialLogMetadata.Last,
				Ordinal: int(buf.PartialLogMetadata.Ordinal),
			}
		}
		msg.Timestamp = time.Unix(0, buf.TimeNano)

		if err := l.Log(&msg); err != nil {
			logrus.WithFields(logrus.Fields{
				"container": l.metadata.containerID,
				"message":   msg,
			}).WithError(err).Error("error writing log message")
			continue
		}

		buf.Reset()
	}
}

func (l *fluentLogger) Log(msg *logger.Message) error {
	data := map[string]string{
		"container_id":   l.metadata.containerID,
		"container_name": l.metadata.containerName,
		"source":         msg.Source,
		"log":            string(msg.Line),
	}
	for k, v := range l.metadata.extra {
		data[k] = v
	}
	if msg.PLogMetaData != nil {
		data["partial_message"] = "true"
		data["partial_id"] = msg.PLogMetaData.ID
		data["partial_ordinal"] = strconv.Itoa(msg.PLogMetaData.Ordinal)
		data["partial_last"] = strconv.FormatBool(msg.PLogMetaData.Last)
	}

	// fluent-logger-golang buffers logs from failures and disconnections,
	// and these are transferred again automatically.
	return l.writer.PostWithTime(l.metadata.tag, msg.Timestamp, data)
}

func (l *fluentLogger) Close() error {
	_ = l.stream.Close()
	return l.writer.Close()
}
