package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/dhenkes/binge-os-watch/internal/model"
	"github.com/dhenkes/binge-os-watch/internal/transport/http/middleware"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writePageJSON[T any](w http.ResponseWriter, page *model.PageResponse[T]) {
	page.EnsureItems()
	writeJSON(w, http.StatusOK, page)
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *model.AppError
	if errors.As(err, &appErr) {
		writeJSON(w, appErr.Code.HTTPStatus(), appErr)
		return
	}
	var valErr *model.ValidationErrors
	if errors.As(err, &valErr) {
		writeJSON(w, http.StatusBadRequest, model.AppError{
			Code:    model.ErrorCodeInvalidArgument,
			Message: valErr.Error(),
			Details: valErr.Fields(),
		})
		return
	}
	slog.Error("internal error", "error", err)
	writeJSON(w, http.StatusInternalServerError, model.AppError{
		Code:    model.ErrorCodeInternal,
		Message: "internal server error",
	})
}

func decodeJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1MB
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		return model.NewInvalidArgument("invalid JSON: " + err.Error())
	}
	return nil
}

func pageRequest(r *http.Request) model.PageRequest {
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	return model.PageRequest{
		PageSize:  size,
		PageToken: r.URL.Query().Get("page_token"),
	}
}

func updateMask(r *http.Request) []string {
	raw := r.URL.Query().Get("update_mask")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

func authUserID(r *http.Request) string {
	return middleware.UserIDFromContext(r.Context())
}
