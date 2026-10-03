package main

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"smartping/src/g"
	applicationHTTP "smartping/src/http"
	"testing"
	"time"

	"github.com/jakecoffman/cron"
)

type shutdownHTTPFixture struct {
	db          *sql.DB
	connection  *sql.Conn
	server      *http.Server
	serveDone   chan struct{}
	requestDone chan struct{}
	requestErr  error
	clientDone  chan struct{}
	clientErr   error
	status      int
	jobs        *backgroundJobs
}

func newShutdownHTTPFixture(t *testing.T) *shutdownHTTPFixture {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TABLE pinglog (logtime TEXT, target TEXT, maxdelay TEXT, mindelay TEXT, avgdelay TEXT, losspk TEXT)`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	connection, err := db.Conn(context.Background())
	if err != nil {
		db.Close()
		t.Fatal(err)
	}
	oldDatabase, oldConfig, oldTimezone := g.Db, g.ConfigSnapshot(), g.LocalTimezone
	g.Db, g.LocalTimezone = db, time.UTC
	g.SetConfig(g.Config{Addr: "127.0.0.1", Network: map[string]g.NetworkMember{}})
	f := &shutdownHTTPFixture{
		db: db, connection: connection, server: applicationHTTP.NewServer(), jobs: newBackgroundJobs(context.Background()),
		serveDone: make(chan struct{}), requestDone: make(chan struct{}), clientDone: make(chan struct{}),
	}
	client := &http.Client{Transport: &http.Transport{}, Timeout: 2 * time.Second}
	t.Cleanup(func() {
		f.server.Close()
		connection.Close()
		f.jobs.Stop()
		client.CloseIdleConnections()
		for _, done := range []chan struct{}{f.requestDone, f.clientDone, f.serveDone} {
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Error("shutdown test request or server did not exit during cleanup")
			}
		}
		db.Close()
		g.Db, g.LocalTimezone = oldDatabase, oldTimezone
		g.SetConfig(oldConfig)
	})
	appHandler := f.server.Handler
	f.server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appHandler.ServeHTTP(w, r)
		f.requestErr = r.Context().Err()
		close(f.requestDone)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		// No request has been started, so cleanup should not wait for it.
		close(f.requestDone)
		close(f.clientDone)
		close(f.serveDone)
		t.Fatal(err)
	}
	go func() {
		defer close(f.serveDone)
		_ = f.server.Serve(listener)
	}()
	go func() {
		defer close(f.clientDone)
		response, err := client.Get("http://" + listener.Addr().String() + "/api/ping.json?ip=192.0.2.1")
		f.clientErr = err
		if err == nil {
			f.status = response.StatusCode
			response.Body.Close()
		}
	}()
	limit := time.Now().Add(time.Second)
	for db.Stats().WaitCount == 0 && time.Now().Before(limit) {
		time.Sleep(time.Millisecond)
	}
	if db.Stats().WaitCount == 0 {
		t.Fatal("Ping request did not queue for the occupied database connection")
	}
	return f
}

func TestShutdownDeadlineCancelsHTTPQueriesWaitingForDatabase(t *testing.T) {
	f := newShutdownHTTPFixture(t)
	scheduler := cron.New()
	scheduler.Start()
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	if err := gracefulShutdown(ctx, f.server, scheduler, f.jobs); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("shutdown error = %v, want original deadline error", err)
	}
	select {
	case <-f.requestDone:
		if !errors.Is(f.requestErr, context.Canceled) {
			t.Errorf("request context error = %v, want canceled", f.requestErr)
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("shutdown timeout left the database query waiting on a live HTTP request")
	}
	select {
	case <-f.clientDone:
		if f.clientErr == nil {
			t.Errorf("client received status %d after forced close, want connection error", f.status)
		}
	case <-time.After(time.Second):
		t.Fatal("client connection remained open after shutdown deadline")
	}
	// Incomplete shutdown must retain the database, even after forcing HTTP
	// connections closed: handlers or background jobs may still be unwinding.
	if err := f.connection.QueryRowContext(context.Background(), "SELECT 1").Scan(new(int)); err != nil {
		t.Fatalf("held connection became unusable: %v", err)
	}
	if err := f.connection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Ping(); err != nil {
		t.Fatalf("database closed after incomplete shutdown: %v", err)
	}
}

func TestShutdownAllowsHTTPQueriesToFinishWithinGracePeriod(t *testing.T) {
	f := newShutdownHTTPFixture(t)
	scheduler := cron.New()
	scheduler.Start()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- gracefulShutdown(ctx, f.server, scheduler, f.jobs) }()
	select {
	case <-f.serveDone:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not stop accepting connections")
	}
	if err := f.connection.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-shutdownDone; err != nil {
		t.Fatalf("shutdown interrupted a request inside its grace period: %v", err)
	}
	<-f.requestDone
	<-f.clientDone
	if f.requestErr != nil || f.clientErr != nil || f.status != http.StatusOK {
		t.Errorf("request result = context %v, client %v, status %d; want normal 200 response", f.requestErr, f.clientErr, f.status)
	}
	if err := f.db.Ping(); err == nil {
		t.Fatal("successful shutdown left the database open")
	}
}
