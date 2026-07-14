package httpx

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

const MaxJSONBodyBytes int64 = 1 << 20

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
	raw, err := io.ReadAll(io.LimitReader(r.Body, MaxJSONBodyBytes+1))
	if err != nil || int64(len(raw)) > MaxJSONBodyBytes {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return false
	}
	var trailing any
	return errors.Is(decoder.Decode(&trailing), io.EOF)
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
