package driver

import (
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/pkg/urlutil"
	"github.com/docker/go-units"
	"github.com/fluent/fluent-logger-golang/fluent"
	"github.com/pkg/errors"
)

const (
	defaultBufferLimit = 1024 * 1024
	defaultHost        = "127.0.0.1"
	defaultPort        = 24224
	defaultProtocol    = "tcp"

	// logger tries to reconnect 2**32 - 1 times
	// failed (and panic) after 204 years [ 1.5 ** (2**32 - 1) - 1 seconds]
	defaultMaxRetries = math.MaxInt32
	defaultRetryWait  = 1000
)

const (
	addressKey            = "fluentd-address"
	bufferLimitKey        = "fluentd-buffer-limit"
	maxRetriesKey         = "fluentd-max-retries"
	requestAckKey         = "fluentd-request-ack"
	retryWaitKey          = "fluentd-retry-wait"
	subSecondPrecisionKey = "fluentd-sub-second-precision"
	forceStopAsyncSendKey = "fluentd-force-stop-async-send"
)

func parseConfig(cfg map[string]string) (fluent.Config, error) {
	var config fluent.Config

	loc, err := parseAddress(cfg[addressKey])
	if err != nil {
		return config, err
	}

	bufferLimit := defaultBufferLimit
	if cfg[bufferLimitKey] != "" {
		bl64, err := units.RAMInBytes(cfg[bufferLimitKey])
		if err != nil {
			return config, err
		}
		bufferLimit = int(bl64)
	}

	retryWait := defaultRetryWait
	if cfg[retryWaitKey] != "" {
		rwd, err := time.ParseDuration(cfg[retryWaitKey])
		if err != nil {
			return config, err
		}
		retryWait = int(rwd.Seconds() * 1000)
	}

	maxRetries := defaultMaxRetries
	if cfg[maxRetriesKey] != "" {
		mr64, err := strconv.ParseUint(cfg[maxRetriesKey], 10, strconv.IntSize)
		if err != nil {
			return config, err
		}
		maxRetries = int(mr64)
	}

	subSecondPrecision := false
	if cfg[subSecondPrecisionKey] != "" {
		if subSecondPrecision, err = strconv.ParseBool(cfg[subSecondPrecisionKey]); err != nil {
			return config, err
		}
	}

	requestAck := false
	if cfg[requestAckKey] != "" {
		if requestAck, err = strconv.ParseBool(cfg[requestAckKey]); err != nil {
			return config, err
		}
	}

	forceStopAsyncSend := true
	if cfg[forceStopAsyncSendKey] != "" {
		if forceStopAsyncSend, err = strconv.ParseBool(cfg[forceStopAsyncSendKey]); err != nil {
			return config, err
		}
	}

	config = fluent.Config{
		FluentPort:         loc.port,
		FluentHost:         loc.host,
		FluentNetwork:      loc.protocol,
		FluentSocketPath:   loc.path,
		BufferLimit:        bufferLimit,
		RetryWait:          retryWait,
		MaxRetry:           maxRetries,
		SubSecondPrecision: subSecondPrecision,
		RequestAck:         requestAck,
		Async:              true,
		ForceStopAsyncSend: forceStopAsyncSend,
	}

	return config, nil
}

type location struct {
	protocol string
	host     string
	port     int
	path     string
}

func parseAddress(address string) (*location, error) {
	if address == "" {
		return &location{
			protocol: defaultProtocol,
			host:     defaultHost,
			port:     defaultPort,
			path:     "",
		}, nil
	}

	protocol := defaultProtocol
	givenAddress := address
	if urlutil.IsTransportURL(address) {
		url, err := url.Parse(address)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid fluentd-address %s", givenAddress)
		}
		// unix and unixgram socket
		if url.Scheme == "unix" || url.Scheme == "unixgram" {
			return &location{
				protocol: url.Scheme,
				host:     "",
				port:     0,
				path:     url.Path,
			}, nil
		}
		// tcp|udp
		protocol = url.Scheme
		address = url.Host
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		if !strings.Contains(err.Error(), "missing port in address") {
			return nil, errors.Wrapf(err, "invalid fluentd-address %s", givenAddress)
		}
		return &location{
			protocol: protocol,
			host:     host,
			port:     defaultPort,
			path:     "",
		}, nil
	}

	portnum, err := strconv.Atoi(port)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid fluentd-address %s", givenAddress)
	}
	return &location{
		protocol: protocol,
		host:     host,
		port:     portnum,
		path:     "",
	}, nil
}
