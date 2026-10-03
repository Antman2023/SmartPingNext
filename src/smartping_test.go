package main

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"smartping/src/funcs"
	"smartping/src/g"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jakecoffman/cron"
)

func TestBackgroundJobsStopCancelsRunningJobsAndRejectsNewWork(t *testing.T) {
	jobs := newBackgroundJobs(context.Background())
	started := make(chan struct{})
	var completed atomic.Bool
	if !jobs.Start(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		completed.Store(true)
	}) {
		t.Fatal("first job should start")
	}
	<-started

	jobs.Stop()
	if jobs.Start(func(context.Context) {}) {
		t.Fatal("job submitted after Stop should be rejected")
	}
	waitCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := jobs.Wait(waitCtx); err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if !completed.Load() {
		t.Fatal("running job did not observe cancellation")
	}
}

func TestBackgroundJobsWaitHonorsContext(t *testing.T) {
	jobs := newBackgroundJobs(context.Background())
	release := make(chan struct{})
	if !jobs.Start(func(context.Context) {
		<-release
	}) {
		t.Fatal("job should start")
	}
	jobs.Stop()

	waitCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := jobs.Wait(waitCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait error = %v, want context canceled", err)
	}

	close(release)
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
	defer cleanupCancel()
	if err := jobs.Wait(cleanupCtx); err != nil {
		t.Fatalf("cleanup Wait returned error: %v", err)
	}
}

func TestBackgroundJobsRejectsNilJob(t *testing.T) {
	jobs := newBackgroundJobs(context.Background())
	defer jobs.Stop()
	if jobs.Start(nil) {
		t.Fatal("nil job should be rejected")
	}
}

func TestBackgroundJobsRejectsWorkAfterParentCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	jobs := newBackgroundJobs(parent)
	cancel()
	defer jobs.Stop()

	if jobs.Start(func(context.Context) {}) {
		t.Fatal("job should be rejected after parent cancellation")
	}
}

func TestShutdownServiceStopsHTTPJobsAndDatabase(t *testing.T) {
	oldDatabase := g.Db
	oldHTTPClient := g.HttpClient
	defer func() {
		g.Db = oldDatabase
		g.HttpClient = oldHTTPClient
	}()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	g.Db = database
	g.HttpClient = &http.Client{}

	jobs := newBackgroundJobs(context.Background())
	jobStarted := make(chan struct{})
	jobStopped := make(chan struct{})
	if !jobs.Start(func(ctx context.Context) {
		close(jobStarted)
		<-ctx.Done()
		close(jobStopped)
	}) {
		t.Fatal("background job should start")
	}
	<-jobStarted

	scheduler := cron.New()
	scheduler.Start()
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	serveErrors := make(chan error, 1)
	go func() {
		serveErrors <- server.Serve(listener)
	}()

	client := &http.Client{Timeout: time.Second}
	response, err := client.Get("http://" + listener.Addr().String())
	if err != nil {
		t.Fatalf("request before shutdown: %v", err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("status before shutdown = %d, want %d", response.StatusCode, http.StatusNoContent)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := shutdownService(shutdownCtx, server, scheduler, jobs); err != nil {
		t.Fatalf("shutdownService returned error: %v", err)
	}

	select {
	case <-jobStopped:
	case <-time.After(time.Second):
		t.Fatal("background job did not stop")
	}
	if err := <-serveErrors; !errors.Is(err, http.ErrServerClosed) {
		t.Fatalf("Serve error = %v, want http.ErrServerClosed", err)
	}
	closedClient := &http.Client{Timeout: 200 * time.Millisecond}
	if response, err := closedClient.Get("http://" + listener.Addr().String()); err == nil {
		_ = response.Body.Close()
		t.Fatal("HTTP listener still accepted requests after shutdown")
	}
	if err := database.Ping(); err == nil {
		t.Fatal("database remained usable after shutdown")
	}
}

func TestGracefulShutdownPropagatesDeadlineAndKeepsDatabaseOpen(t *testing.T) {
	oldDatabase := g.Db
	oldHTTPClient := g.HttpClient
	defer func() {
		g.Db = oldDatabase
		g.HttpClient = oldHTTPClient
	}()

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer database.Close()
	g.Db = database
	g.HttpClient = nil

	jobs := newBackgroundJobs(context.Background())
	jobStarted := make(chan struct{})
	releaseJob := make(chan struct{})
	if !jobs.Start(func(context.Context) {
		close(jobStarted)
		<-releaseJob
	}) {
		t.Fatal("background job should start")
	}
	<-jobStarted

	scheduler := cron.New()
	scheduler.Start()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err = gracefulShutdown(shutdownCtx, &http.Server{}, scheduler, jobs)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("gracefulShutdown error = %v, want deadline exceeded", err)
	}
	if !strings.Contains(err.Error(), "graceful shutdown incomplete") {
		t.Fatalf("gracefulShutdown error lacks context: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("database closed while a background job was still running: %v", err)
	}

	close(releaseJob)
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
	defer cleanupCancel()
	if err := jobs.Wait(cleanupCtx); err != nil {
		t.Fatalf("cleanup Wait returned error: %v", err)
	}
}

type queuedStorageContext struct {
	context.Context
	once    sync.Once
	waiting chan struct{}
}

func (ctx *queuedStorageContext) Done() <-chan struct{} {
	ctx.once.Do(func() { close(ctx.waiting) })
	return ctx.Context.Done()
}

func TestShutdownServiceCancelsJobsQueuedForDatabaseWriteAccess(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	oldDatabase := g.Db
	g.Db = database
	defer func() { g.Db = oldDatabase }()
	jobs := newBackgroundJobs(context.Background())
	g.DLock.Lock()
	defer func() {
		jobs.Stop()
		g.DLock.Unlock()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := jobs.Wait(cleanupCtx); err != nil {
			t.Errorf("cleanup queued jobs: %v", err)
		}
	}()
	operations := []func(context.Context){
		func(ctx context.Context) { funcs.PingStorageContext(ctx, g.PingSt{}, "192.0.2.1", "2026-10-04 12:00") },
		func(ctx context.Context) { _ = funcs.AlertStorageContext(ctx, g.AlertLog{}) },
		func(ctx context.Context) { _ = funcs.MapPingStorageContext(ctx) },
		funcs.ClearArchiveContext,
	}
	for _, operation := range operations {
		waiting := make(chan struct{})
		if !jobs.Start(func(ctx context.Context) { operation(&queuedStorageContext{Context: ctx, waiting: waiting}) }) {
			t.Fatal("queued storage job did not start")
		}
		select {
		case <-waiting:
		case <-time.After(time.Second):
			t.Fatal("storage job did not queue for database write access")
		}
	}
	scheduler := cron.New()
	scheduler.Start()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := shutdownService(shutdownCtx, &http.Server{}, scheduler, jobs); err != nil {
		t.Fatalf("queued storage prevented shutdown while the write lock remained held: %v", err)
	}
	if jobs.Start(func(context.Context) {}) {
		t.Fatal("shutdown accepted new background work")
	}
	if err := database.Ping(); err == nil {
		t.Fatal("database remained open after queued jobs exited")
	}
}
