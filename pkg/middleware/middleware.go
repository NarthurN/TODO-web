package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/NarthurN/TODO-API-web/pkg/loger"
	"github.com/golang-jwt/jwt/v5"
)

var secret []byte = []byte("my_secret_key")

var HelperNow = time.Now
var HelperSince = time.Since

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Создаём обёртку поверх оригинального ResponseWriter
		lrw := &loggingResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK, // значение по умолчанию
		}

		start := HelperNow()
		next.ServeHTTP(w, r)
		duration := HelperSince(start)

		loger.L.Info(fmt.Sprintf(
			"[%s] %s %s %d %v",
			r.Method,
			r.RequestURI,
			r.RemoteAddr,
			lrw.statusCode,
			duration,
		))
	})
}

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// смотрим наличие пароля
		expectedPass := os.Getenv("TODO_PASSWORD")
		if len(expectedPass) > 0 {
			var jwtToken string // JWT-токен из куки
			// получаем куку
			cookie, err := r.Cookie("token")
			if err != nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}
			jwtToken = cookie.Value

			token, err := jwt.Parse(jwtToken, func(t *jwt.Token) (interface{}, error) {
				return []byte(os.Getenv("TODO_JWT_SECRET")), nil
			})
			if err != nil {
				loger.L.Error("failed to parse token:", "err", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			if !token.Valid {
				// возвращаем ошибку авторизации 401
				http.Error(w, "Authentification required", http.StatusUnauthorized)
				return
			}

			res, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				loger.L.Error("failed to typecast to jwt.MapClaims:", "err", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			hashPassRaw, ok := res["password"]
			if !ok {
				loger.L.Error("no hashPassword in payload:")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			hashPass, ok := hashPassRaw.(string)
			if !ok {
				loger.L.Error("failed to typecast to string")
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}

			expectedPass32bytes := sha256.Sum256([]byte(expectedPass))
			expectedPassHash := hex.EncodeToString(expectedPass32bytes[:])

			if hashPass != expectedPassHash {
				loger.L.Error("passwords are not same")
				http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
