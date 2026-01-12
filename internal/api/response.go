package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/shaiso/Automata/internal/repo"
)

// ErrorCode — код ошибки API.
type ErrorCode string

const (
	ErrCodeBadRequest     ErrorCode = "BAD_REQUEST"
	ErrCodeNotFound       ErrorCode = "NOT_FOUND"
	ErrCodeConflict       ErrorCode = "CONFLICT"
	ErrCodeInvalidState   ErrorCode = "INVALID_STATE"
	ErrCodeInternalError  ErrorCode = "INTERNAL_ERROR"
	ErrCodeMethodNotAllow ErrorCode = "METHOD_NOT_ALLOWED"
)

// ErrorResponse — структура ответа с ошибкой.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail — детали ошибки.
type ErrorDetail struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

// DataResponse — структура успешного ответа.
type DataResponse struct {
	Data any `json:"data"`
}

// ListResponse — структура ответа со списком.
type ListResponse struct {
	Data  any `json:"data"`
	Total int `json:"total,omitempty"`
}

// JSON отправляет JSON ответ.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Success отправляет успешный ответ с данными.
func Success(w http.ResponseWriter, data any) {
	JSON(w, http.StatusOK, DataResponse{Data: data})
}

// Created отправляет ответ о создании ресурса.
func Created(w http.ResponseWriter, data any) {
	JSON(w, http.StatusCreated, DataResponse{Data: data})
}

// NoContent отправляет ответ без тела (204).
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// List отправляет ответ со списком.
func List(w http.ResponseWriter, data any, total int) {
	JSON(w, http.StatusOK, ListResponse{Data: data, Total: total})
}

// Error отправляет ответ с ошибкой.
func Error(w http.ResponseWriter, status int, code ErrorCode, message string) {
	JSON(w, status, ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	})
}

// BadRequest отправляет ошибку 400.
func BadRequest(w http.ResponseWriter, message string) {
	Error(w, http.StatusBadRequest, ErrCodeBadRequest, message)
}

// NotFound отправляет ошибку 404.
func NotFound(w http.ResponseWriter, message string) {
	Error(w, http.StatusNotFound, ErrCodeNotFound, message)
}

// Conflict отправляет ошибку 409.
func Conflict(w http.ResponseWriter, message string) {
	Error(w, http.StatusConflict, ErrCodeConflict, message)
}

// InvalidState отправляет ошибку 422.
func InvalidState(w http.ResponseWriter, message string) {
	Error(w, http.StatusUnprocessableEntity, ErrCodeInvalidState, message)
}

// InternalError отправляет ошибку 500.
func InternalError(w http.ResponseWriter, logger *slog.Logger, err error) {
	logger.Error("internal error", "error", err)
	Error(w, http.StatusInternalServerError, ErrCodeInternalError, "internal server error")
}

// MethodNotAllowed отправляет ошибку 405.
func MethodNotAllowed(w http.ResponseWriter) {
	Error(w, http.StatusMethodNotAllowed, ErrCodeMethodNotAllow, "method not allowed")
}

// HandleRepoError преобразует ошибку репозитория в HTTP ответ.
func HandleRepoError(w http.ResponseWriter, logger *slog.Logger, err error, notFoundMsg string) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, repo.ErrNotFound) {
		NotFound(w, notFoundMsg)
		return true
	}

	if errors.Is(err, repo.ErrInvalidState) {
		InvalidState(w, err.Error())
		return true
	}

	InternalError(w, logger, err)
	return true
}
