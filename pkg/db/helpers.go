package db

import (
	"time"

	"github.com/NarthurN/TODO-API-web/pkg/api"
)

const (
	InputLayout = "02.01.2006"
)

func IsDate(search string) (string, bool) {
	if search == "" {
		return search, false
	}
	date, err := time.Parse(InputLayout, search)
	if err != nil {
		return search, false
	}

	return date.Format(api.Layout), true
}
