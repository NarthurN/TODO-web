package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/NarthurN/TODO-API-web/pkg/loger"
	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidJSONFormat error = errors.New("invalid JSON format")
var ErrTitleIsEmpty error = errors.New("пустой заголовок")
var ErrInvalidDate error = errors.New("date is in invalid format")
var ErrIncorrectPassword error = errors.New("неверный пароль")

type Storage interface {
	GetTasks(limit int, search string) ([]Task, error)
	AddTask(task Task) (int64, error)
	GetTask(id string) (*Task, error)
	UpdateTask(task *Task) error
	DeleteTask(id string) error
	UpdateDate(next string, id string) error
	Close() error
}

type Api struct {
	Storage Storage
}

func New(db Storage) *Api {
	return &Api{Storage: db}
}

func (h *Api) NextDayHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nowStr := r.FormValue("now")
		dateStr := r.FormValue("date")
		repeat := r.FormValue("repeat")
		loger.L.Info("FormValue:", "nowStr", nowStr)
		loger.L.Info("FormValue:", "dateStr", dateStr)
		loger.L.Info("FormValue:", "repeat", repeat)
		var now time.Time
		if nowStr == "" {
			now = time.Now().UTC()
		} else {
			var err error
			now, err = time.Parse(Layout, nowStr)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
		}

		if dateStr == "" || repeat == "" {
			http.Error(w, "Missing parameters: now, date or repeat", http.StatusBadRequest)
			return
		}

		newDate, err := NextDate(now, dateStr, repeat)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		loger.L.Info("Ответ newDate:", "newDate", newDate)
		fmt.Fprint(w, newDate)
	})
}

func (h *Api) AddTaskHandle() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var task Task
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			loger.L.Error(ErrInvalidJSONFormat.Error())
			SendErrorResponse(w, ErrInvalidJSONFormat.Error())
			return
		}

		if task.Title == "" {
			loger.L.Error(ErrTitleIsEmpty.Error())
			SendErrorResponse(w, ErrTitleIsEmpty.Error())
			return
		}

		err := checkDate(&task)
		if err != nil {
			loger.L.Error(ErrTitleIsEmpty.Error())
			SendErrorResponse(w, ErrTitleIsEmpty.Error())
			return
		}

		id, err := h.Storage.AddTask(task)
		loger.L.Info("Отпраляем id", "id", id)
		task.ID = strconv.Itoa(int(id))
		if err != nil {
			loger.L.Error(err.Error())
			SendErrorResponse(w, err.Error())
			return
		}

		if err = json.NewEncoder(w).Encode(task); err != nil {
			loger.L.Info("Отпраляем id", "id", id)
			SendIdResponse(w, id)
			return
		}
	})
}

func (h *Api) GetTasksHandle() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		search := r.URL.Query().Get("search")
		tasks, err := h.Storage.GetTasks(50, search) // в параметре максимальное количество записей
		if err != nil {
			SendErrorResponse(w, err.Error())
			return
		}
		WriteJSON(w, TasksResponse{
			Tasks: tasks,
		})
	})
}

func (h *Api) GetTaskHandle() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			loger.L.Error("no id provided")
			SendErrorResponse(w, "Не указан идентификатор")
			return
		}

		task, err := h.Storage.GetTask(id)
		if err != nil {
			loger.L.Error("failed to get task", "id", id, "error", err)
			if strings.Contains(err.Error(), "no task with id") {
				SendErrorResponse(w, "Задача не найдена")
			} else {
				SendErrorResponse(w, "Ошибка сервера")
			}
			return
		}

		loger.L.Info("task retrieved successfully", "id", id)
		WriteJSON(w, task)
	})
}

func (h *Api) ChangeTaskHandle() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var task Task
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			loger.L.Error(ErrInvalidJSONFormat.Error())
			SendErrorResponse(w, ErrInvalidJSONFormat.Error())
			return
		}

		if task.Title == "" {
			loger.L.Error(ErrTitleIsEmpty.Error())
			SendErrorResponse(w, ErrTitleIsEmpty.Error())
			return
		}

		err := checkDate(&task)
		if err != nil {
			loger.L.Error(ErrTitleIsEmpty.Error())
			SendErrorResponse(w, ErrTitleIsEmpty.Error())
			return
		}

		err = h.Storage.UpdateTask(&task)
		if err != nil {
			loger.L.Error("failed to update task", "id", task.ID, "error", err)
			if strings.Contains(err.Error(), "no task found with id") {
				SendErrorResponse(w, "Задача не найдена")
			} else {
				SendErrorResponse(w, "Ошибка сервера")
			}
			return
		}

		loger.L.Info("task updated successfully", "id", task.ID)
		WriteJSON(w, struct{}{})
	})
}

func (h *Api) DeleteTaskHandle() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			loger.L.Error("no id provided")
			SendErrorResponse(w, "Не указан идентификатор")
			return
		}

		if err := h.Storage.DeleteTask(id); err != nil {
			loger.L.Error("no id provided")
			SendErrorResponse(w, "Нет пользователя с этим ID")
			return
		}

		loger.L.Info("task deleted successfully", "id", id)
		WriteJSON(w, struct{}{})
	})
}

func (h *Api) DeleteOrRepeatHandle() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("id")
		if id == "" {
			loger.L.Error("no id provided")
			SendErrorResponse(w, "Не указан идентификатор")
			return
		}

		task, err := h.Storage.GetTask(id)
		if err != nil {
			loger.L.Error("cannot do h.Storage.GetTask", "err", err)
			SendErrorResponse(w, "Нет пользователя с этим ID")
			return
		}

		const NoRepeatRule = ""
		switch task.Repeat {
		case NoRepeatRule:
			loger.L.Info("Delete task", "id", task.ID, "repeat", task.Repeat)
			if err := h.Storage.DeleteTask(id); err != nil {
				loger.L.Error("no id provided")
				SendErrorResponse(w, "Нет задачи с этим ID")
				return
			}
			loger.L.Info("task deleted successfully", "id", id)
		default:
			loger.L.Info("Update task", "id", task.ID, "repeat", task.Repeat)
			newDate, err := NextDate(time.Now(), task.Date, task.Repeat)
			if err != nil {
				loger.L.Error("h.Storage.DeleteTask:", "err", err)
				SendErrorResponse(w, "Невозможно обновить задачу")
				return
			}

			if err := h.Storage.UpdateDate(newDate, id); err != nil {
				loger.L.Error("h.Storage.UpdateDate:", "err", err)
				SendErrorResponse(w, "Невозможно обновить задачу")
				return
			}
			loger.L.Info("task updated successfully", "id", id)
		}

		WriteJSON(w, struct{}{})
	})
}

func (h *Api) SignInHandle() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPassword := os.Getenv("TODO_PASSWORD")
		loger.L.Info("expectedPassword:", "expectedPassword", expectedPassword)

		password := struct {
			Pass string `json:"password"`
		}{}

		if err := json.NewDecoder(r.Body).Decode(&password); err != nil {
			loger.L.Error(" json.NewDecoder(r.Body).Decode:", "err", err)
			SendErrorResponse(w, "Невозможно преобразовать пароль")
			return
		}
		loger.L.Info("password.Pass:", "password.Pass", password.Pass)
		if password.Pass != expectedPassword {
			loger.L.Error(" json.NewDecoder(r.Body).Decode:", "err", ErrIncorrectPassword)
			SendErrorResponse(w, ErrIncorrectPassword.Error())
			return
		}

		hashPass32bytes := sha256.Sum256([]byte(password.Pass))
		hashPass := hex.EncodeToString(hashPass32bytes[:])

		claims := &jwt.MapClaims{
			"password": hashPass,
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		signedToken, err := token.SignedString([]byte(os.Getenv("TODO_JWT_SECRET")))
		if err != nil {
			loger.L.Error("token.SignedString:", "err", err)
			SendErrorResponse(w, http.StatusText(http.StatusInternalServerError))
			return
		}

		tokenResponse := struct {
			Token string `json:"token"`
		}{
			Token: signedToken,
		}

		loger.L.Info("Сформирован token:", "token", signedToken)
		WriteJSON(w, tokenResponse)
	})
}
