package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/NarthurN/TODO-API-web/internal/config"
	"github.com/NarthurN/TODO-API-web/pkg/api"
	"github.com/NarthurN/TODO-API-web/pkg/loger"
	_ "modernc.org/sqlite"
)

type TaskStorage struct {
	SqlStorage *sql.DB
}

func New() (*TaskStorage, error) {
	dbFile := config.Cfg.TODO_DBFILE
	db, err := sql.Open("sqlite", dbFile)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: cannot open database: %w", err)
	}

	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(5)
	db.SetConnMaxIdleTime(time.Minute * 5)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db.Ping: failed to ping database: %w", err)
	}

	storage := &TaskStorage{SqlStorage: db}

	if err := createTable(storage); err != nil {
		return nil, fmt.Errorf("createTable: cannot create table: %w", err)
	}

	return storage, nil
}

func createTable(storage *TaskStorage) error {
	_, err := storage.SqlStorage.Exec(`
		CREATE TABLE IF NOT EXISTS scheduler (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date CHAR(8) NOT NULL DEFAULT "",
			title VARCHAR(256) NOT NULL DEFAULT "",
			comment TEXT NOT NULL DEFAULT "",
			repeat VARCHAR(128) NOT NULL DEFAULT ""
		);
		CREATE INDEX IF NOT EXISTS scheduler_date ON scheduler (date);
	`)
	if err != nil {
		return fmt.Errorf("storage.SqlStorage.Exec: failed to create scheduler table: %w", err)
	}

	return nil
}

func (t *TaskStorage) Close() error {
	err := t.SqlStorage.Close()
	if err != nil {
		return fmt.Errorf("t.SqlStorage.Close: error closing db: %w", err)
	}
	return nil
}

func (t *TaskStorage) AddTask(task api.Task) (int64, error) {
	res, err := t.SqlStorage.Exec(`INSERT INTO scheduler (date, title, comment, repeat) VALUES (:date, :title, :comment, :repeat)`,
		sql.Named("date", task.Date),
		sql.Named("title", task.Title),
		sql.Named("comment", task.Comment),
		sql.Named("repeat", task.Repeat))
	if err != nil {
		return 0, fmt.Errorf("t.SqlStorage.Exec: error by inserting task: %w", err)
	}

	return res.LastInsertId()
}

func (t *TaskStorage) GetTasks(limit int, rowSearch string) ([]api.Task, error) {
	loger.L.Info("Зпрос search", "search", rowSearch)
	tasks := make([]api.Task, 0)
	var rows *sql.Rows
	search, ok := IsDate(rowSearch)
	if ok {
		var err error
		rows, err = t.SqlStorage.Query(`SELECT * FROM scheduler WHERE date = :search LIMIT :limit `,
			sql.Named("search", search),
			sql.Named("limit", limit))
		if err != nil {
			return nil, fmt.Errorf("t.SqlStorage.Query: cannot do SELECT: %w", err)
		}
	} else {
		var err error
		rows, err = t.SqlStorage.Query(`
			SELECT * FROM scheduler 
			WHERE title LIKE '%' || :search || '%'
			OR comment LIKE '%' || :search || '%'
			ORDER BY date 
			LIMIT :limit`,
			sql.Named("search", search),
			sql.Named("limit", limit))
		if err != nil {
			return nil, fmt.Errorf("t.SqlStorage.Query: cannot do SELECT: %w", err)
		}
	}

	defer rows.Close()

	for rows.Next() {
		task := api.Task{}

		err := rows.Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
		if err != nil {
			return nil, fmt.Errorf("rows.Scan: cannot do Scan: %w", err)
		}

		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows.Err: err in rows: %w", err)
	}
	loger.L.Info("Получили tasks", "tasks", tasks)
	return tasks, nil
}

func (t *TaskStorage) GetTask(id string) (*api.Task, error) {
	if id == "" {
		loger.L.Error("invalid task ID", "id", id)
		return nil, fmt.Errorf("invalid task ID: %s", id)
	}

	task := &api.Task{}
	err := t.SqlStorage.QueryRow(
		"SELECT id, date, title, comment, repeat FROM scheduler WHERE id = :id",
		sql.Named("id", id),
	).Scan(&task.ID, &task.Date, &task.Title, &task.Comment, &task.Repeat)
	if err == sql.ErrNoRows {
		loger.L.Error("no task found", "id", id)
		return task, fmt.Errorf("no task with id %s", id)
	}
	if err != nil {
		loger.L.Error("failed to query task", "id", id, "error", err)
		return task, fmt.Errorf("t.SqlStorage.QueryRow: failed to get task with id %s: %w", id, err)
	}

	loger.L.Info("task retrieved", "id", id)
	return task, nil
}

func (t *TaskStorage) UpdateTask(task *api.Task) error {
	if task.ID == "" {
		loger.L.Error("invalid task ID", "id", task.ID)
		return fmt.Errorf("invalid task ID: %s", task.ID)
	}

	result, err := t.SqlStorage.Exec(`
        UPDATE scheduler 
        SET date = :date, title = :title, comment = :comment, repeat = :repeat 
        WHERE id = :id`,
		sql.Named("date", task.Date),
		sql.Named("title", task.Title),
		sql.Named("comment", task.Comment),
		sql.Named("repeat", task.Repeat),
		sql.Named("id", task.ID))
	if err != nil {
		loger.L.Error("failed to update task", "id", task.ID, "error", err)
		return fmt.Errorf("t.SqlStorage.Exec: failed to update task with id %s: %w", task.ID, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		loger.L.Error("failed to get rows affected", "id", task.ID, "error", err)
		return fmt.Errorf("result.RowsAffected: failed to check rows affected for id %s: %w", task.ID, err)
	}
	if rowsAffected == 0 {
		loger.L.Error("no task found", "id", task.ID)
		return fmt.Errorf("no task found with id %s", task.ID)
	}

	loger.L.Info("task updated successfully", "id", task.ID)
	return nil
}

func (t *TaskStorage) DeleteTask(id string) error {
	res, err := t.SqlStorage.Exec("DELETE FROM scheduler WHERE id = :id", sql.Named("id", id))
	if err != nil {
		loger.L.Error("failed to delete task", "id", id, "error", err)
		return fmt.Errorf("t.SqlStorage.Exec: failed to delete task with id %s: %w", id, err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		loger.L.Error("failed to get rows affected", "id", id, "error", err)
		return fmt.Errorf("result.RowsAffected: failed to check rows affected for id %s: %w", id, err)
	}
	if rowsAffected == 0 {
		loger.L.Error("no task found", "id", id)
		return fmt.Errorf("no task found with id %s", id)
	}

	loger.L.Info("task deleted successfully", "id", id)
	return nil
}

func (t *TaskStorage) UpdateDate(next string, id string) error {
	result, err := t.SqlStorage.Exec(`
        UPDATE scheduler 
        SET date = :date 
        WHERE id = :id`,
		sql.Named("date", next),
		sql.Named("id", id))
	if err != nil {
		loger.L.Error("failed to update task", "id", id, "error", err)
		return fmt.Errorf("t.SqlStorage.Exec: failed to update task with id %s: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		loger.L.Error("failed to get rows affected", "id", id, "error", err)
		return fmt.Errorf("result.RowsAffected: failed to check rows affected for id %s: %w", id, err)
	}
	if rowsAffected == 0 {
		loger.L.Error("no task found", "id", id)
		return fmt.Errorf("no task found with id %s", id)
	}

	loger.L.Info("task updated successfully", "id", id)
	return nil
}
