package validator

import (
	"fmt"
	"net/http"

	"be_region_indonesia/pkg/response"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

type CustomValidator struct {
	validator *validator.Validate
}

func NewCustomValidator() *CustomValidator {
	return &CustomValidator{validator: validator.New()}
}

func formatIndonesianValidationError(fe validator.FieldError) string {
	field := fe.Field()
	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("Field '%s' wajib diisi.", field)
	case "numeric":
		return fmt.Sprintf("Field '%s' harus berupa karakter angka (numerik).", field)
	case "min":
		return fmt.Sprintf("Field '%s' minimal harus memiliki %s karakter.", field, fe.Param())
	case "max":
		return fmt.Sprintf("Field '%s' maksimal boleh memiliki %s karakter.", field, fe.Param())
	case "len":
		return fmt.Sprintf("Field '%s' harus memiliki panjang tepat %s karakter.", field, fe.Param())
	default:
		return fmt.Sprintf("Field '%s' tidak memenuhi aturan validasi '%s'.", field, fe.Tag())
	}
}

func (cv *CustomValidator) Validate(i interface{}) error {
	if err := cv.validator.Struct(i); err != nil {
		var fieldErrors []response.FieldError
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			for _, vErr := range validationErrors {
				fieldErrors = append(fieldErrors, response.FieldError{
					Field: vErr.Field(),
					Issue: formatIndonesianValidationError(vErr),
				})
			}
		} else {
			fieldErrors = append(fieldErrors, response.FieldError{
				Field: "payload",
				Issue: "Format body permintaan tidak valid atau JSON rusak.",
			})
		}
		return echo.NewHTTPError(http.StatusUnprocessableEntity, fieldErrors)
	}
	return nil
}
