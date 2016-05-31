package server

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/tsileo/blobstash/pkg/blobstore"
	"github.com/tsileo/blobstash/pkg/hub"
	"github.com/tsileo/blobstash/pkg/kvstore"
	"github.com/tsileo/blobstash/pkg/meta"
	"github.com/tsileo/blobstash/pkg/middleware"
	"github.com/tsileo/blobstash/pkg/nsdb"
	"github.com/tsileo/blobstash/pkg/synctable"

	"github.com/gorilla/mux"
	log "github.com/inconshreveable/log15"
)

type App interface {
	Register(*mux.Router)
}

type Server struct {
	router    *mux.Router
	log       log.Logger
	closeFunc func() error
}

func New() (*Server, error) {
	logger := log.New()
	logger.SetHandler(log.StreamHandler(os.Stdout, log.TerminalFormat()))
	s := &Server{
		router: mux.NewRouter(),
		log:    logger,
	}
	hub := hub.New(logger.New("app", "hub"))
	// Load the blobstore
	blobstore, err := blobstore.New(logger.New("app", "blobstore"), hub)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blobstore app: %v", err)
	}
	// FIXME(tsileo): handle middleware in the `Register` interface
	blobstore.Register(s.router.PathPrefix("/api/blobstore").Subrouter())
	// Load the meta
	metaHandler, err := meta.New(logger.New("app", "meta"), blobstore, hub)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blobstore meta: %v", err)
	}
	// Load the kvstore
	kvstore, err := kvstore.New(logger.New("app", "kvstore"), blobstore, metaHandler)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize kvstore app: %v", err)
	}
	kvstore.Register(s.router.PathPrefix("/api/kvstore").Subrouter())
	nsDB, err := nsdb.New(logger.New("app", "nsdb"), "/Users/thomas/var/blobstash/nsdb", blobstore, metaHandler, hub)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize nsdb: %v", err)
	}
	// Load the synctable
	synctable := synctable.New(logger.New(), blobstore, nsDB)
	synctable.Register(s.router.PathPrefix("/api/sync").Subrouter())

	// Setup the closeFunc
	s.closeFunc = func() error {
		if err := blobstore.Close(); err != nil {
			return err
		}
		if err := kvstore.Close(); err != nil {
			return err
		}
		if err := nsDB.Close(); err != nil {
			return err
		}
		return nil
	}
	return s, nil
}

func (s *Server) Serve() error {
	go func() {
		http.ListenAndServe(":8051", middleware.Secure(s.router))
	}()
	s.tillShutdown()
	return s.closeFunc()
	// return http.ListenAndServe(":8051", s.router)
}

func (s *Server) tillShutdown() {
	// Listen for shutdown signal
	cs := make(chan os.Signal, 1)
	signal.Notify(cs, os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	for {
		select {
		case sig := <-cs:
			s.log.Debug("captured signal", "signal", sig)
			s.log.Info("shutting down...")
			return
		}
	}
}
