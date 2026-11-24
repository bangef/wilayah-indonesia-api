package handler

import (
	"errors"
	"net/http"

	"be_region_indonesia/internal/domain"
	"be_region_indonesia/pkg/response"
	"github.com/labstack/echo/v4"
)

type RegionWriteHandler struct {
	uc domain.RegionUsecase
}

func NewRegionWriteHandler(uc domain.RegionUsecase) *RegionWriteHandler {
	return &RegionWriteHandler{uc: uc}
}

type CreateProvinceRequest struct {
	Code string `json:"code" validate:"required,numeric,max=10"`
	Name string `json:"name" validate:"required,min=2,max=100"`
	Type string `json:"type" validate:"omitempty,max=50"`
}

type CreateChildRegionRequest struct {
	Code string `json:"code" validate:"required,max=100"`
	Name string `json:"name" validate:"required,min=2,max=100"`
	Type string `json:"type" validate:"omitempty,max=50"`
}

type CreatePostalCodeRequest struct {
	PostalCode string `json:"postal_code" validate:"required,numeric,len=5"`
}

type UpdateRegionRequest struct {
	Name string `json:"name" validate:"required,min=2,max=100"`
	Type string `json:"type" validate:"omitempty,max=50"`
}

// CreateProvince handles POST /api/v1/provinces
// @Summary      Tambah Provinsi Baru
// @Description  Membuat entitas provinsi baru dengan kode 2 digit numerik unik. Memerlukan autentikasi JWT Admin.
// @Tags         Admin Mutations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      CreateProvinceRequest  true  "Payload data provinsi baru"
// @Success      201      {object}  response.ResponseEnvelope{data=domain.Region}
// @Failure      400      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      409      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /provinces [post]
func (h *RegionWriteHandler) CreateProvince(c echo.Context) error {
	var req CreateProvinceRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload permintaan tidak valid.", response.FieldError{Field: "body", Issue: err.Error()})
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	reg, err := h.uc.CreateProvince(c.Request().Context(), req.Code, req.Name, req.Type)
	if err != nil {
		return h.handleError(c, err)
	}

	return response.Success(c, http.StatusCreated, "Provinsi baru berhasil ditambahkan.", reg, nil)
}

// CreateRegency handles POST /api/v1/provinces/:code/regencies
// @Summary      Tambah Kabupaten/Kota Baru
// @Description  Membuat entitas kabupaten/kota baru di bawah provinsi induk. Memerlukan autentikasi JWT Admin.
// @Tags         Admin Mutations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        code     path      string                    true  "Kode Provinsi Induk (contoh: 11)"
// @Param        payload  body      CreateChildRegionRequest  true  "Payload kabupaten/kota (format code: XX.YY)"
// @Success      201      {object}  response.ResponseEnvelope{data=domain.Region}
// @Failure      400      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      404      {object}  response.ResponseEnvelope
// @Failure      409      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /provinces/{code}/regencies [post]
func (h *RegionWriteHandler) CreateRegency(c echo.Context) error {
	parentCode := c.Param("code")
	return h.handleCreateChild(c, parentCode, domain.LevelRegency, "Kabupaten/kota baru berhasil ditambahkan.")
}

// CreateDistrict handles POST /api/v1/regencies/:code/districts
// @Summary      Tambah Kecamatan Baru
// @Description  Membuat entitas kecamatan baru di bawah kabupaten/kota induk. Memerlukan autentikasi JWT Admin.
// @Tags         Admin Mutations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        code     path      string                    true  "Kode Kabupaten/Kota Induk (contoh: 11.01)"
// @Param        payload  body      CreateChildRegionRequest  true  "Payload kecamatan (format code: XX.YY.ZZ)"
// @Success      201      {object}  response.ResponseEnvelope{data=domain.Region}
// @Failure      400      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      404      {object}  response.ResponseEnvelope
// @Failure      409      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /regencies/{code}/districts [post]
func (h *RegionWriteHandler) CreateDistrict(c echo.Context) error {
	parentCode := c.Param("code")
	return h.handleCreateChild(c, parentCode, domain.LevelDistrict, "Kecamatan baru berhasil ditambahkan.")
}

// CreateVillage handles POST /api/v1/districts/:code/villages
// @Summary      Tambah Kelurahan/Desa Baru
// @Description  Membuat entitas kelurahan/desa baru di bawah kecamatan induk. Memerlukan autentikasi JWT Admin.
// @Tags         Admin Mutations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        code     path      string                    true  "Kode Kecamatan Induk (contoh: 11.01.01)"
// @Param        payload  body      CreateChildRegionRequest  true  "Payload kelurahan/desa (format code: XX.YY.ZZ.WWWW)"
// @Success      201      {object}  response.ResponseEnvelope{data=domain.Region}
// @Failure      400      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      404      {object}  response.ResponseEnvelope
// @Failure      409      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /districts/{code}/villages [post]
func (h *RegionWriteHandler) CreateVillage(c echo.Context) error {
	parentCode := c.Param("code")
	return h.handleCreateChild(c, parentCode, domain.LevelVillage, "Kelurahan/desa baru berhasil ditambahkan.")
}

// CreatePostalCode handles POST /api/v1/villages/:code/postal-codes
// @Summary      Tambah Kode Pos pada Kelurahan/Desa
// @Description  Menautkan nomor kode pos 5 digit ke kelurahan/desa induk. Memerlukan autentikasi JWT Admin.
// @Tags         Admin Mutations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        code     path      string                   true  "Kode Kelurahan/Desa Induk (contoh: 11.01.01.2001)"
// @Param        payload  body      CreatePostalCodeRequest  true  "Payload kode pos (5 digit numerik)"
// @Success      201      {object}  response.ResponseEnvelope{data=domain.Region}
// @Failure      400      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      404      {object}  response.ResponseEnvelope
// @Failure      409      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /villages/{code}/postal-codes [post]
func (h *RegionWriteHandler) CreatePostalCode(c echo.Context) error {
	villageCode := c.Param("code")
	var req CreatePostalCodeRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload permintaan tidak valid.", response.FieldError{Field: "body", Issue: err.Error()})
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	reg, err := h.uc.CreatePostalCode(c.Request().Context(), villageCode, req.PostalCode)
	if err != nil {
		return h.handleError(c, err)
	}

	return response.Success(c, http.StatusCreated, "Kode pos berhasil ditambahkan dan ditautkan ke kelurahan/desa.", reg, nil)
}

// UpdateRegion handles PUT /api/v1/regions/:code
// @Summary      Perbarui Data Wilayah
// @Description  Memperbarui atribut nama atau tipe wilayah berdasarkan kode unik. Memerlukan autentikasi JWT Admin.
// @Tags         Admin Mutations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        code     path      string               true  "Kode Wilayah yang akan diperbarui"
// @Param        payload  body      UpdateRegionRequest  true  "Payload pembaharuan wilayah"
// @Success      200      {object}  response.ResponseEnvelope{data=domain.Region}
// @Failure      400      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      404      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /regions/{code} [put]
func (h *RegionWriteHandler) UpdateRegion(c echo.Context) error {
	code := c.Param("code")
	var req UpdateRegionRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload permintaan tidak valid.", response.FieldError{Field: "body", Issue: err.Error()})
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	reg, err := h.uc.UpdateRegion(c.Request().Context(), code, req.Name, req.Type)
	if err != nil {
		return h.handleError(c, err)
	}

	return response.Success(c, http.StatusOK, "Data wilayah berhasil diperbarui.", reg, nil)
}

// UpdateVillagePostalCode handles PUT /api/v1/villages/:code/postal-code
// @Summary      Perbarui Kode Pos Kelurahan/Desa
// @Description  Memperbarui atau menautkan kode pos 5 digit langsung pada kelurahan/desa. Memerlukan autentikasi JWT Admin.
// @Tags         Admin Mutations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        code     path      string                   true  "Kode Kelurahan/Desa (contoh: 11.01.01.2001)"
// @Param        payload  body      CreatePostalCodeRequest  true  "Payload kode pos baru (5 digit)"
// @Success      200      {object}  response.ResponseEnvelope
// @Failure      400      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      404      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /villages/{code}/postal-code [put]
func (h *RegionWriteHandler) UpdateVillagePostalCode(c echo.Context) error {
	code := c.Param("code")
	var req CreatePostalCodeRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload permintaan tidak valid.", response.FieldError{Field: "body", Issue: err.Error()})
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	err := h.uc.UpdatePostalCodeByVillageCode(c.Request().Context(), code, req.PostalCode)
	if err != nil {
		return h.handleError(c, err)
	}

	return response.Success(c, http.StatusOK, "Kode pos kelurahan berhasil diperbarui.", nil, nil)
}

// DeleteRegion handles DELETE /api/v1/regions/:code
// @Summary      Hapus Data Wilayah (Soft Delete)
// @Description  Menandai wilayah sebagai dihapus. Ditolak jika wilayah masih memiliki anak aktif. Memerlukan autentikasi JWT Admin.
// @Tags         Admin Mutations
// @Produce      json
// @Security     BearerAuth
// @Param        code     path      string  true  "Kode Wilayah yang akan dihapus"
// @Success      200      {object}  response.ResponseEnvelope
// @Failure      401      {object}  response.ResponseEnvelope
// @Failure      404      {object}  response.ResponseEnvelope
// @Failure      409      {object}  response.ResponseEnvelope
// @Failure      500      {object}  response.ResponseEnvelope
// @Router       /regions/{code} [delete]
func (h *RegionWriteHandler) DeleteRegion(c echo.Context) error {
	code := c.Param("code")
	err := h.uc.DeleteRegion(c.Request().Context(), code)
	if err != nil {
		return h.handleError(c, err)
	}
	return response.Success(c, http.StatusOK, "Data wilayah berhasil dihapus (soft delete).", nil, nil)
}

func (h *RegionWriteHandler) handleCreateChild(c echo.Context, parentCode string, targetLevel int16, msg string) error {
	var req CreateChildRegionRequest
	if err := c.Bind(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload permintaan tidak valid.", response.FieldError{Field: "body", Issue: err.Error()})
	}
	if err := c.Validate(&req); err != nil {
		return err
	}

	reg, err := h.uc.CreateChildRegion(c.Request().Context(), parentCode, req.Code, req.Name, req.Type, targetLevel)
	if err != nil {
		return h.handleError(c, err)
	}

	return response.Success(c, http.StatusCreated, msg, reg, nil)
}

func (h *RegionWriteHandler) handleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, domain.ErrRegionNotFound), errors.Is(err, domain.ErrParentNotFound):
		return response.Error(c, http.StatusNotFound, "Data wilayah tidak ditemukan.", response.FieldError{Field: "code", Issue: err.Error()})
	case errors.Is(err, domain.ErrRegionCodeAlreadyExists):
		return response.Error(c, http.StatusConflict, "Terjadi konflik pada data wilayah.", response.FieldError{Field: "code", Issue: err.Error()})
	case errors.Is(err, domain.ErrInvalidChildPrefix), errors.Is(err, domain.ErrInvalidLevel):
		return response.Error(c, http.StatusBadRequest, "Format payload permintaan tidak valid.", response.FieldError{Field: "code", Issue: err.Error()})
	case errors.Is(err, domain.ErrCannotDeleteWithActiveChildren):
		return response.Error(c, http.StatusConflict, "Terjadi konflik pada data wilayah.", response.FieldError{Field: "children", Issue: err.Error()})
	default:
		return response.Error(c, http.StatusInternalServerError, "Terjadi kesalahan internal pada server.", response.FieldError{Field: "server", Issue: err.Error()})
	}
}
