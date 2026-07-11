package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

type Envelope struct {
	OK    bool        `json:"ok"`
	Data  any         `json:"data,omitempty"`
	Error *ErrorBody  `json:"error,omitempty"`
	Meta  interface{} `json:"meta,omitempty"`
}

type ErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func OK(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusOK, Envelope{OK: true, Data: data})
}

func Created(w http.ResponseWriter, data any) {
	writeJSON(w, http.StatusCreated, Envelope{OK: true, Data: data})
}

func Error(w http.ResponseWriter, status int, code int, message string) {
	writeJSON(w, status, Envelope{OK: false, Error: &ErrorBody{Code: code, Message: message}})
}

func Decode(r *http.Request, dst any) bool {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst) == nil
}

func AccountID(r *http.Request) (int64, error) {
	raw := r.Header.Get("X-Account-Id")
	if raw == "" {
		raw = r.URL.Query().Get("accountId")
	}
	if raw == "" {
		return 0, errors.New("accountId is required")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("accountId is invalid")
	}
	return id, nil
}

func CharacterID(r *http.Request) (int64, error) {
	raw := r.Header.Get("X-Character-Id")
	if raw == "" {
		raw = r.URL.Query().Get("characterId")
	}
	if raw == "" {
		return 0, errors.New("characterId is required")
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("characterId is invalid")
	}
	return id, nil
}

func writeJSON(w http.ResponseWriter, status int, body Envelope) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
