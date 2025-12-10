package httperror

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/render"
)

type HTTPError struct {
	error
	Code    int    `json:"code"`
	Message string `json:"error"`
}

func (e *HTTPError) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.Code)
	return nil
}

func New(code int, message string, cause error) *HTTPError {
	return &HTTPError{
		error:   cause,
		Code:    code,
		Message: message,
	}
}

func InternalServerError(message string, err error) *HTTPError {
	return New(http.StatusInternalServerError, "internal server error", fmt.Errorf("%s: %w", message, err))
}

func Forbidden(message string) *HTTPError {
	return New(http.StatusForbidden, message, errors.New(message))
}

func NotFound(message string) *HTTPError {
	return New(http.StatusNotFound, message, errors.New(message))
}

func BadRequest(message string) *HTTPError {
	return New(http.StatusBadRequest, message, errors.New(message))
}

func BadRequestWithError(message string, err error) *HTTPError {
	return New(http.StatusBadRequest, message, fmt.Errorf("%s: %w", message, err))
}
