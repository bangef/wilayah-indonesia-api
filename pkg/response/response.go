package response

import (
	"math"
	"net/http"

	"github.com/labstack/echo/v4"
)

type Meta struct {
	Page         int   `json:"page,omitempty"`
	Limit        int   `json:"limit,omitempty"`
	TotalRecords int64 `json:"total_records,omitempty"`
	TotalPages   int   `json:"total_pages,omitempty"`
}

type FieldError struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

type ResponseEnvelope struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Data    interface{}  `json:"data,omitempty"`
	Meta    *Meta        `json:"meta,omitempty"`
	Errors  []FieldError `json:"errors,omitempty"`
}

func Success(c echo.Context, httpStatus int, message string, data interface{}, meta *Meta) error {
	return c.JSON(httpStatus, ResponseEnvelope{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

func SuccessPaginated(c echo.Context, message string, data interface{}, page, limit int, total int64) error {
	totalPages := 0
	if limit > 0 {
		totalPages = int(math.Ceil(float64(total) / float64(limit)))
	}
	return Success(c, http.StatusOK, message, data, &Meta{
		Page:         page,
		Limit:        limit,
		TotalRecords: total,
		TotalPages:   totalPages,
	})
}

func Error(c echo.Context, httpStatus int, message string, errors ...FieldError) error {
	return c.JSON(httpStatus, ResponseEnvelope{
		Success: false,
		Message: message,
		Errors:  errors,
	})
}
