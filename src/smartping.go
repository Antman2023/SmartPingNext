package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	standardHTTP "net/http"
	"os"
	"os/signal"
	"smartping/src/funcs"
	"smartping/src/g"
	applicationHTTP "smartping/src/http"
	"smartping/src/nettools"
	"sync"
	"syscall"
	"time"

	"github.com/jakecoffman/cron"
	"github.com/sirupsen/logrus"
)

// Init config
var Version = "dev"

const shutdownTimeout = 15 * time.Second

type backgroundJobs struct {
	ctx      context.Context
	cancel   context.CancelFunc
	mu       sync.Mutex
	wg       sync.WaitGroup
	stopping bool
}

func newBackgroundJobs(parent context.Context) *backgroundJobs {
	ctx, cancel := context.WithCancel(parent)
	return &backgroundJobs{ctx: ctx, cancel: cancel}
}

func (jobs *backgroundJobs) Start(job func(context.Context)) bool {
	if job == nil {
		return false
	}
	jobs.mu.Lock()
	if jobs.stopping || jobs.ctx.Err() != nil {
		jobs.mu.Unlock()
		return false
	}
	jobs.wg.Add(1)
	ctx := jobs.ctx
	jobs.mu.Unlock()

	go func() {
		defer jobs.wg.Done()
		job(ctx)
	}()
	return true
}

func (jobs *backgroundJobs) Stop() {
	jobs.mu.Lock()
	if !jobs.stopping {
		jobs.stopping = true
		jobs.cancel()
	}
	jobs.mu.Unlock()
}

func (jobs *backgroundJobs) Wait(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		jobs.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	version := flag.Bool("v", false, "show version")
	flag.Parse()
	if *version {
		fmt.Println(Version)
		os.Exit(0)
	}
	g.ParseConfig(Version)
	if err := runService(); err != nil {
		log.Fatal(err)
	}
}

func runService() error {
	serviceCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	jobs := newBackgroundJobs(serviceCtx)
	jobs.Start(funcs.ClearArchiveContext)

	c := cron.New()
	c.AddFunc("*/60 * * * * *", func() {
		jobs.Start(funcs.PingContext)
		jobs.Start(funcs.MappingContext)
		if g.ConfigSnapshot().Mode["Type"] == "cloud" {
			jobs.Start(funcs.StartCloudMonitorContext)
		}
	}, "ping")
	c.AddFunc("0 0 * * * *", func() {
		jobs.Start(funcs.ClearArchiveContext)
	}, "mtc")
	c.Start()

	server := applicationHTTP.NewServer()
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case <-serviceCtx.Done():
		logrus.Info("Shutdown signal received")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := shutdownService(shutdownCtx, server, c, jobs); err != nil {
			logrus.Warn("Graceful shutdown incomplete: ", err)
		}
		return nil
	case err := <-serverErrors:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		shutdownErr := shutdownService(shutdownCtx, server, c, jobs)
		if errors.Is(err, standardHTTP.ErrServerClosed) {
			return shutdownErr
		}
		return errors.Join(fmt.Errorf("HTTP server failed: %w", err), shutdownErr)
	}
}

func shutdownService(ctx context.Context, server *standardHTTP.Server, scheduler *cron.Cron, jobs *backgroundJobs) error {
	scheduler.Stop()
	jobs.Stop()

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Shutdown(ctx)
	}()
	jobsErr := jobs.Wait(ctx)
	serverErr := <-serverErrors
	if serverErr != nil || jobsErr != nil {
		return errors.Join(serverErr, jobsErr)
	}

	if g.HttpClient != nil {
		g.HttpClient.CloseIdleConnections()
	}
	poolErr := nettools.CloseICMPPool()
	var databaseErr error
	if g.Db != nil {
		databaseErr = g.Db.Close()
	}
	return errors.Join(poolErr, databaseErr)
}
