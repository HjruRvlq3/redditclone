package server

import (
	"encoding/json"
	"errors"
	"net/http"

	"redditclone/internal/jwtconfig"
	"redditclone/internal/store"

	"golang.org/x/crypto/bcrypt"
)

// postLogin логин
func postLogin(userStore *store.UserStore, jwtManager *jwtconfig.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// принять JSON
		var req registerReq

		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid json")
			return
		}

		// проверить name и password
		user, err := userStore.GetByUsername(req.Username)
		if err != nil {
			if errors.Is(err, store.ErrUserNotFound) {
				writeJSONError(w, http.StatusUnauthorized, "user not found")
				return
			}
			writeJSONError(w, http.StatusInternalServerError, "server error")
			return
		}

		err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
		if err != nil {
			writeJSONError(w, http.StatusUnauthorized, "invalid password")
			return
		}

		// токен
		token, err := jwtManager.GenerateToken(user)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "cannot sign token")
			return
		}

		// отдать, что залогинен
		writeJSONResponse(w, http.StatusOK, map[string]string{"token": token})
	}
}
