package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/TIBCOSoftware/flogo-lib/engine"
	"github.com/op/go-logging"
)

func init() {
	var format = logging.MustStringFormatter(
		"%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.5s} %{color:reset} %{message}",
	)

	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)
	logging.SetLevel(logging.INFO, "")
}

var log = logging.MustGetLogger("main")

func main() {

	engineConfig := GetEngineConfig()
	triggersConfig := GetTriggersConfig()

	logLevel, _ := logging.LogLevel(engineConfig.LogLevel)
	logging.SetLevel(logLevel, "")

	engine := engine.NewEngine(engineConfig, triggersConfig)

	EnableFlowServices(engine, engineConfig)

	engine.Start()

	exitChan := setupSignalHandling()

	code := <-exitChan

	engine.Stop()

	os.Exit(code)
}

func setupSignalHandling() chan int {

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	exitChan := make(chan int)
	go func() {
		for {
			s := <-signalChan
			switch s {
			// kill -SIGHUP
			case syscall.SIGHUP:
				exitChan <- 0
			// kill -SIGINT/Ctrl+c
			case syscall.SIGINT:
				exitChan <- 0
			// kill -SIGTERM
			case syscall.SIGTERM:
				exitChan <- 0
			// kill -SIGQUIT
			case syscall.SIGQUIT:
				exitChan <- 0
			default:
				log.Debug("Unknown signal.")
				exitChan <- 1
			}
		}
	}()

	return exitChan
}
