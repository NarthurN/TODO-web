package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NarthurN/TODO-API-web/internal/config"
	"github.com/NarthurN/TODO-API-web/pkg/api"
	"github.com/NarthurN/TODO-API-web/pkg/loger"
	"github.com/NarthurN/TODO-API-web/pkg/middleware"
)

type Server struct {
	GoServer *http.Server
}

func (s *Server) Run() error {
	serverError := make(chan error, 1)

	go func() {
		loger.L.Info("Сервер слушает по адресу", "addr", s.GoServer.Addr)
		if err := s.GoServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverError <- fmt.Errorf("s.GoServer.ListenAndServe: server error: %w", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverError:
		return err
	case sig := <-sigChan:
		loger.L.Info("Received signal, shutting down...", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.GoServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("s.GoServer.Shutdown: graceful shutdown failed: %w", err)
		}
		loger.L.Info("Server stopped gracefully")
		return nil
	}
}

type storage interface {
	AddTask(task api.Task) (int64, error)
	GetTasks(limit int, search string) ([]api.Task, error)
	GetTask(id string) (*api.Task, error)
	UpdateTask(task *api.Task) error
	DeleteTask(id string) error
	UpdateDate(next string, id string) error
	Close() error
}

func New(db storage) *Server {
	return &Server{
		GoServer: &http.Server{
			Addr:           ":" + config.Cfg.TODO_PORT,
			Handler:        NewMux(db),
			ReadTimeout:    10 * time.Second,
			WriteTimeout:   10 * time.Second,
			MaxHeaderBytes: 1 << 20,
		},
	}
}

func NewMux(db storage) http.Handler {
	mux := http.NewServeMux()
	api := api.New(db)

	mux.Handle(`GET /`, http.FileServer(http.Dir(`./web`)))
	// "api/nextdate?now=20240126&date=20240126&repeat=y"
	mux.Handle("GET /api/nextdate", api.NextDayHandler())

	mux.Handle("GET /api/tasks", middleware.Auth(api.GetTasksHandle()))
	mux.Handle("POST /api/task", middleware.Auth(api.AddTaskHandle()))

	// GET /api/task?id=<идентификатор>
	mux.Handle("GET /api/task", middleware.Auth(api.GetTaskHandle()))
	mux.Handle("PUT /api/task", middleware.Auth(api.ChangeTaskHandle()))
	// /api/task/done?id=<идентификатор>
	mux.Handle("POST /api/task/done", middleware.Auth(api.DeleteOrRepeatHandle()))
	// /api/task?id=<идентификатор>
	mux.Handle("DELETE /api/task", middleware.Auth(api.DeleteTaskHandle()))

	//аутентификация
	mux.Handle("POST /api/signin", api.SignInHandle())

	wrappedMux := middleware.Logging(mux)

	return wrappedMux
}
