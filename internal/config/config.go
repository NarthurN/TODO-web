package config

import (
	"os"

	"go1f/pkg/loger"

	"github.com/joho/godotenv"
)

var Cfg *Config

type Config struct {
	TODO_PORT   string
	TODO_DBFILE string
}

func Init() {
	Cfg = &Config{}
	if err := godotenv.Load(); err != nil {
		loger.L.Error("Error loading .env file")
		os.Exit(1)
	}

	Cfg.TODO_PORT = os.Getenv("TODO_PORT")
	if Cfg.TODO_PORT == "" {
		Cfg.TODO_PORT = "7540"
	}

	Cfg.TODO_DBFILE = os.Getenv("TODO_DBFILE")
	if Cfg.TODO_DBFILE == "" {
		Cfg.TODO_DBFILE = "scheduler.db"
	}
}
