package loger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

var L *slog.Logger

type PrettyHandler struct {
	slog.Handler
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	// Форматируем время
	timeStr := r.Time.Format("2006-01-02 15:04:05")

	// Цвета для уровней
	var level string
	switch r.Level {
	case slog.LevelDebug:
		level = colorize("DEBUG", 36) // cyan
	case slog.LevelInfo:
		level = colorize("INFO", 32) // green
	case slog.LevelWarn:
		level = colorize("WARN", 33) // yellow
	case slog.LevelError:
		level = colorize("ERROR", 31) // red
	default:
		level = r.Level.String()
	}

	// Собираем атрибуты
	var attrs string
	r.Attrs(func(a slog.Attr) bool {
		attrs += fmt.Sprintf(" %s=%s", colorize(a.Key, 34), a.Value)
		return true
	})

	// Выводим в формате: [TIME] LEVEL MESSAGE ATTRS
	fmt.Printf("%s %s %s%s\n",
		colorize(timeStr, 90), // серый
		level,
		colorize(r.Message, 1), // жирный
		attrs,
	)
	return nil
}

func colorize(s interface{}, color int) string {
	return fmt.Sprintf("\033[%dm%v\033[0m", color, s)
}

func Init() {
	L = slog.New(&PrettyHandler{
		Handler: slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
			AddSource: true,
		}),
	})
}
