package main

import (
	"encoding/json"
	"net/http"
	"os"
	"runtime/pprof"

	"github.com/NiR-/fluentd-async-logger/driver"
	"github.com/docker/docker/daemon/logger"
	"github.com/docker/go-plugins-helpers/sdk"
	"github.com/sirupsen/logrus"
)

func main() {
	if logLevelCfg, ok := os.LookupEnv("LOG_LEVEL"); ok {
		logLevel, err := logrus.ParseLevel(logLevelCfg)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to parse log level.")
		}
		logrus.SetLevel(logLevel)
	}

	h := sdk.NewHandler(`{"Implements": ["LoggingDriver"]}`)
	setUpHandlers(&h, driver.New())

	if debug, _ := os.LookupEnv("DEBUG"); debug != "" {
		h.HandleFunc("/pprof/trace", func(w http.ResponseWriter, r *http.Request) {
			_ = pprof.Lookup("goroutine").WriteTo(w, 1)
		})
	}

	logrus.Info("Start serving on the UNIX socket...")
	if err := h.ServeUnix("fluentd-async", 0); err != nil {
		panic(err)
	}
}

func setUpHandlers(h *sdk.Handler, d *driver.FluentDriver) {
	h.HandleFunc("/LogDriver.StartLogging", func(w http.ResponseWriter, r *http.Request) {
		var req driver.StartLoggingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err := d.StartLogging(req)
		respond(err, w)
	})

	h.HandleFunc("/LogDriver.StopLogging", func(w http.ResponseWriter, r *http.Request) {
		var req driver.StopLoggingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err := d.StopLogging(req)
		respond(err, w)
	})

	h.HandleFunc("/LogDriver.Capabilities", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(struct {
			Cap logger.Capability
		}{
			Cap: logger.Capability{ReadLogs: false},
		})
	})

	h.HandleFunc("/LogDriver.ReadLogs", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not implemented", http.StatusNotImplemented)
	})
}

func respond(err error, w http.ResponseWriter) {
	var rawErr string
	if err != nil {
		rawErr = err.Error()
	}

	_ = json.NewEncoder(w).Encode(struct {
		Err string
	}{
		Err: rawErr,
	})
}
