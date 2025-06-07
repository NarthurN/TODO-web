package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/NarthurN/TODO-API-web/pkg/loger"
)

const Layout = "20060102"

const (
	day   = "d"
	year  = "y"
	week  = "w"
	month = "m"
)

var (
	ErrInvalidRepeatParameter error = errors.New("arg repeat is empty")
	ErrUnknownFormat          error = errors.New("unknown format in repeat")
	ErrManyDays               error = errors.New("days are more than 400")
	ErrManyWeeks              error = errors.New("weeks are more than 7")
	ErrManyMonths             error = errors.New("months are more than 12")
	ErrInvalidFormatInDay     error = errors.New("format of day is incorrect")
	ErrInvalidFormatInMonth   error = errors.New("format of month is incorrect")
)

func NextDate(now time.Time, dstart string, repeat string) (string, error) {
	if repeat == "" {
		return "", ErrInvalidRepeatParameter
	}
	repeatSlice := strings.Split(repeat, " ")

	timeDstart, err := time.Parse(Layout, dstart)
	if err != nil {
		return "", fmt.Errorf("time.Parse: cannot parse dstart: %w", err)
	}

	switch repeatSlice[0] {
	case day:
		if len(repeatSlice) < 2 {
			return "", ErrInvalidFormatInDay
		}
		days, err := strconv.Atoi(repeatSlice[1])
		if err != nil {
			return "", fmt.Errorf("strconv.Atoi: cannot convet string to int: %w", err)
		}

		if days > 400 {
			return "", ErrManyDays
		}

		for {
			timeDstart = timeDstart.AddDate(0, 0, days)
			if timeDstart.After(now) {
				break
			}
		}
	case year:
		for {
			timeDstart = time.Date(timeDstart.Year()+1, timeDstart.Month(), timeDstart.Day(), 0, 0, 0, 0, timeDstart.Location())
			if timeDstart.Equal(now) || timeDstart.After(now) {
				break
			}
		}
	case week:
		if len(repeatSlice) < 2 {
			return "", ErrInvalidRepeatParameter
		}
		daysOfWeekStr := strings.Split(repeatSlice[1], ",")
		var daysInt []int
		for _, dayStr := range daysOfWeekStr {
			dayInt, err := strconv.Atoi(dayStr)
			if err != nil {
				return "", fmt.Errorf("strconv.Atoi: cannot convet string to int: %w", err)
			}
			if dayInt > 7 {
				return "", ErrManyWeeks
			}
			daysInt = append(daysInt, dayInt-1)
		}
		i := int(timeDstart.Weekday())
		for {
			timeDstart = timeDstart.AddDate(0, 0, 1)
			if slices.Contains(daysInt, i) && timeDstart.After(now) {
				break
			}
			i = (i + 1) % 7
		}
	case month:
		daysOfMonthsStr := strings.Split(repeatSlice[1], ",")
		var monthsOfYearStr []string
		if len(repeatSlice) > 2 {
			monthsOfYearStr = strings.Split(repeatSlice[2], ",")
		}
		var daysOfMonthsInts []int
		var monthsOfYearInts []int
		var includeLastDay bool
		var includePreLastDay bool
		for _, daysOfMonths := range daysOfMonthsStr {
			dayOfMonthInt, err := strconv.Atoi(daysOfMonths)
			if err != nil {
				return "", fmt.Errorf("strconv.Atoi: cannot convet string to int: %w", err)
			}
			switch {
			case dayOfMonthInt == -1:
				includeLastDay = true
			case dayOfMonthInt == -2:
				includePreLastDay = true
			case dayOfMonthInt < 1 || dayOfMonthInt > 31:
				return "", ErrInvalidFormatInMonth
			default:
				daysOfMonthsInts = append(daysOfMonthsInts, dayOfMonthInt)
			}
		}
		if len(monthsOfYearStr) == 0 {
			monthsOfYearInts = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
		} else {
			for _, monthOfYearStr := range monthsOfYearStr {
				monthOfYearInt, err := strconv.Atoi(monthOfYearStr)
				if err != nil {
					return "", fmt.Errorf("strconv.Atoi: cannot convet string to int: %w", err)
				}
				if monthOfYearInt > 12 {
					return "", ErrManyMonths
				}
				monthsOfYearInts = append(monthsOfYearInts, monthOfYearInt)
			}
			sort.Ints(monthsOfYearInts)
		}

		current := timeDstart
		for yearOffset := 0; yearOffset < 10; yearOffset++ {
			year := current.Year() + yearOffset
			startMonth := 1
			if yearOffset == 0 {
				startMonth = int(current.Month())
			}

			for _, monthInt := range monthsOfYearInts {
				if yearOffset == 0 && monthInt < startMonth {
					continue
				}

				month := time.Month(monthInt)
				daysToCheck := make([]int, len(daysOfMonthsInts))
				copy(daysToCheck, daysOfMonthsInts)

				if includeLastDay {
					lastDay := LastDayOfMonth(time.Date(year, month, 1, 0, 0, 0, 0, current.Location()))
					daysToCheck = append(daysToCheck, lastDay.Day())
				}
				if includePreLastDay {
					preLastDay := PreLastDayOfMonth(time.Date(year, month, 1, 0, 0, 0, 0, current.Location()))
					daysToCheck = append(daysToCheck, preLastDay.Day())
				}

				sort.Ints(daysToCheck)

				for _, day := range daysToCheck {
					testDate := time.Date(year, month, day, 0, 0, 0, 0, current.Location())
					if testDate.Month() != month {
						continue
					}
					if testDate.After(now) {
						return testDate.Format("20060102"), nil
					}
				}
			}
		}

	default:
		return "", ErrUnknownFormat
	}

	return timeDstart.Format(Layout), nil
}

func LastDayOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location()).Add(-24 * time.Hour)
}

func PreLastDayOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, t.Location()).Add(-48 * time.Hour)
}

func SendErrorResponse(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	response := Response{
		Error: errorMsg,
	}
	loger.L.Info("Response sent", "response", response)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

func SendIdResponse(w http.ResponseWriter, id int64) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := Response{
		ID: id,
	}
	loger.L.Info("Response sent", "response", response)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

func WriteJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	loger.L.Info("Response sent", "response", data)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Failed to encode JSON", http.StatusInternalServerError)
	}
}

func checkDate(task *Task) error {
	now := time.Now()

	if task.Date == "" {
		task.Date = now.Format(Layout)
		return nil
	}

	t, err := time.Parse(Layout, task.Date)
	if err != nil {
		return ErrInvalidDate
	}

	var next string
	if task.Repeat != "" {
		next, err = NextDate(now, task.Date, task.Repeat)
		if err != nil {
			return fmt.Errorf("NextDate: cannot get NextDate: %w", err)
		}
	}

	if afterNow(now, t) {
		if task.Repeat == "" {
			task.Date = now.Format(Layout)
		} else {
			task.Date = next
		}
	}

	return nil
}

func afterNow(now, t time.Time) bool {
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return nowDate.After(tDate)
}

func IsDate(search string) (string, bool) {
	date, err := time.Parse("02.01.2006", search)
	if err != nil {
		return search, false
	}
	return date.Format("20060102"), true
}
