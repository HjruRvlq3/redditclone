package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"redditclone/internal/jwtconfig"
	"redditclone/internal/model"
	"redditclone/internal/store"

	"golang.org/x/crypto/bcrypt"
)

// FieldError ошибка валидации конкретного поля
type FieldError struct {
	Location string `json:"location"`
	Param    string `json:"param"`
	Msg      string `json:"msg"`
}

// ValidationErrorResponse массив ошибок
type ValidationErrorResponse struct {
	Errors []FieldError `json:"errors"`
}

var usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// postRegister регистрация
func postRegister(userStore *store.UserStore, jwtManager *jwtconfig.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// получить данные от клиента
		var req registerReq

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json")
			return
		}

		// созадть пользователя
		// валидация
		var valErrors []FieldError
		if len(req.Username) < 3 || len(req.Username) > 32 {
			valErrors = append(valErrors, FieldError{
				Location: "body",
				Param:    "username",
				Msg:      "must be between 3 and 32 characters",
			})
		} else if !usernameRegex.MatchString(req.Username) {
			valErrors = append(valErrors, FieldError{
				Location: "body",
				Param:    "username",
				Msg:      "can only contain letters, numbers, underscores and hyphens",
			})
		}

		if len(req.Password) < 8 {
			valErrors = append(valErrors, FieldError{
				Location: "body",
				Param:    "password",
				Msg:      "must be at least 8 characters",
			})
		}

		if len(valErrors) > 0 {
			writeJSONResponse(w, http.StatusUnprocessableEntity, ValidationErrorResponse{Errors: valErrors})
			return
		}

		// внесение в список юзеров
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		user := model.NewUser(req.Username, string(hash))

		if err := userStore.AddUser(user); err != nil {
			if errors.Is(err, store.ErrUserAlreadyExists) {
				writeJSONResponse(w, http.StatusUnprocessableEntity, ValidationErrorResponse{Errors: []FieldError{
					{Location: "body", Param: "username", Msg: "already exists"},
				}})
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		// отдать jwt
		token, err := jwtManager.GenerateToken(user)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "cannot sign token")
			return
		}

		// отдать "успех"
		writeJSONResponse(w, http.StatusCreated, map[string]string{"token": token})
	}
}
