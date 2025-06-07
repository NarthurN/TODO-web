package main

import (
	"os"

	"github.com/NarthurN/TODO-API-web/internal/config"
	"github.com/NarthurN/TODO-API-web/internal/server"
	"github.com/NarthurN/TODO-API-web/pkg/db"
	"github.com/NarthurN/TODO-API-web/pkg/loger"
)

func main() {
	loger.Init()
	config.Init()

	db, err := db.New()
	if err != nil {
		loger.L.Error("db.New: creating database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	server := server.New(db)

	if err := server.Run(); err != nil {
		loger.L.Error("Ошибка при запуске сервера", "err", err)
		os.Exit(1)
	}
}
