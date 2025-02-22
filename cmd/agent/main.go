package main

import (
	//Its important that we do this first so that we can register with the windows service control ASAP to avoid timeouts
	_ "github.com/grafana/agent/cmd/agent/initiate"

	"flag"
	"log"
	"os"

	util_log "github.com/cortexproject/cortex/pkg/util/log"
	"github.com/go-kit/log/level"
	initLog "github.com/grafana/agent/cmd/agent/log"
	"github.com/grafana/agent/pkg/config"

	// Adds version information
	_ "github.com/grafana/agent/pkg/build"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"

	// Register Prometheus SD components
	_ "github.com/grafana/loki/clients/pkg/promtail/discovery/consulagent"
	_ "github.com/prometheus/prometheus/discovery/install"

	// Register integrations
	_ "github.com/grafana/agent/pkg/integrations/install"
)

func init() {
	prometheus.MustRegister(version.NewCollector("agent"))
}
func main() {
	// If flow is enabled go into that working mode
	// TODO allow flow to run as a windows service
	if isFlowEnabled() {
		runFlow()
		return
	}
	reloader := func() (*config.Config, error) {
		fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		return config.Load(fs, os.Args[1:])
	}
	cfg, err := reloader()
	if err != nil {
		log.Fatalln(err)
	}
	// After this point we can start using go-kit logging.
	logger := initLog.NewLogger(&cfg.Server)
	util_log.Logger = logger
	ep, err := NewEntrypoint(logger, cfg, reloader)
	if err != nil {
		level.Error(logger).Log("msg", "error creating the agent server entrypoint", "err", err)
		os.Exit(1)
	}
	if err = ep.Start(); err != nil {
		level.Error(logger).Log("msg", "error running agent", "err", err)
		// Don't os.Exit here; we want to do cleanup by stopping promMetrics
	}
	ep.Stop()
	level.Info(logger).Log("msg", "agent exiting")
}
