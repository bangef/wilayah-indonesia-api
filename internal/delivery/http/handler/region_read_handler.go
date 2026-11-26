package handler

import (
	"net/http"
	"strconv"

	"be_region_indonesia/internal/domain"
	"be_region_indonesia/pkg/response"
	"be_region_indonesia/pkg/sanitizer"

	"github.com/labstack/echo/v4"
)

type RegionReadHandler struct {
	uc domain.RegionUsecase
}

func NewRegionReadHandler(uc domain.RegionUsecase) *RegionReadHandler {
	return &RegionReadHandler{uc: uc}
}

// GetProvinces handles GET /api/v1/provinces
// @Summary      Daftar Provinsi
// @Description  Mengambil daftar provinsi dengan pagination, pencarian nama, dan pengurutan dinamis.
// @Tags         Provinces
// @Produce      json
// @Param        page     query    int     false  "Nomor halaman (default: 1)"
// @Param        limit    query    int     false  "Jumlah data per halaman (default: 20, max: 100)"
// @Param        search   query    string  false  "Filter pencarian nama provinsi"
// @Param        order    query    string  false  "Format pengurutan data: code:asc, code:desc, name:asc, name:desc"
// @Success      200      {object} response.ResponseEnvelope{data=[]domain.Region,meta=response.Meta}
// @Failure      400      {object} response.ResponseEnvelope
// @Failure      500      {object} response.ResponseEnvelope
// @Router       /provinces [get]
func (h *RegionReadHandler) GetProvinces(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	search := c.QueryParam("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 1000 {
		limit = 20
	}

	orderBy, orderDir, err := sanitizer.ParseOrderParam(
		c.QueryParam("order"),
		c.QueryParam("order_by"),
		c.QueryParam("order_dir"),
	)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload permintaan tidak valid.", response.FieldError{
			Field: "order",
			Issue: err.Error(),
		})
	}

	items, total, err := h.uc.GetProvinces(c.Request().Context(), domain.FilterParams{
		Search:   search,
		Page:     page,
		Limit:    limit,
		OrderBy:  orderBy,
		OrderDir: orderDir,
	})
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "Gagal mengambil data wilayah dari basis data.", response.FieldError{Field: "database", Issue: err.Error()})
	}

	return response.SuccessPaginated(c, "Data provinsi berhasil diambil.", items, page, limit, total)
}

// GetProvinceByCode handles GET /api/v1/provinces/:code
// @Summary      Detail Provinsi
// @Description  Mengambil informasi detail satu provinsi berdasarkan kode wilayah unik 2 digit.
// @Tags         Provinces
// @Produce      json
// @Param        code   path     string  true  "Kode Provinsi (contoh: 11, 31, 32)"
// @Success      200    {object} response.ResponseEnvelope{data=domain.Region}
// @Failure      404    {object} response.ResponseEnvelope
// @Failure      500    {object} response.ResponseEnvelope
// @Router       /provinces/{code} [get]
func (h *RegionReadHandler) GetProvinceByCode(c echo.Context) error {
	code := c.Param("code")
	reg, err := h.uc.GetByCode(c.Request().Context(), code)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "Provinsi tidak ditemukan.", response.FieldError{Field: "code", Issue: err.Error()})
	}
	return response.Success(c, http.StatusOK, "Data provinsi berhasil ditemukan.", reg, nil)
}

// GetRegenciesByProvince handles GET /api/v1/provinces/:code/regencies
// @Summary      Daftar Kabupaten/Kota per Provinsi
// @Description  Mengambil daftar kabupaten dan kota di bawah provinsi induk tertentu dengan pagination, search, dan sorting.
// @Tags         Regencies
// @Produce      json
// @Param        code     path     string  true   "Kode Provinsi Induk (contoh: 11)"
// @Param        page     query    int     false  "Nomor halaman (default: 1)"
// @Param        limit    query    int     false  "Jumlah data per halaman (default: 20, max: 100)"
// @Param        search   query    string  false  "Filter pencarian nama kabupaten/kota"
// @Param        order    query    string  false  "Format pengurutan data: code:asc, code:desc, name:asc, name:desc"
// @Success      200      {object} response.ResponseEnvelope{data=[]domain.Region,meta=response.Meta}
// @Failure      400      {object} response.ResponseEnvelope
// @Failure      404      {object} response.ResponseEnvelope
// @Failure      500      {object} response.ResponseEnvelope
// @Router       /provinces/{code}/regencies [get]
func (h *RegionReadHandler) GetRegenciesByProvince(c echo.Context) error {
	parentCode := c.Param("code")
	return h.handleChildrenQuery(c, parentCode, domain.LevelRegency, "Data kabupaten/kota berhasil diambil.")
}

// GetRegencyByCode handles GET /api/v1/regencies/:code
// @Summary      Detail Kabupaten/Kota
// @Description  Mengambil informasi detail satu kabupaten/kota berdasarkan kode wilayah unik (format: XX.YY).
// @Tags         Regencies
// @Produce      json
// @Param        code   path     string  true  "Kode Kabupaten/Kota (contoh: 11.01, 31.71)"
// @Success      200    {object} response.ResponseEnvelope{data=domain.Region}
// @Failure      404    {object} response.ResponseEnvelope
// @Failure      500    {object} response.ResponseEnvelope
// @Router       /regencies/{code} [get]
func (h *RegionReadHandler) GetRegencyByCode(c echo.Context) error {
	code := c.Param("code")
	reg, err := h.uc.GetByCode(c.Request().Context(), code)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "Kabupaten/kota tidak ditemukan.", response.FieldError{Field: "code", Issue: err.Error()})
	}
	return response.Success(c, http.StatusOK, "Data kabupaten/kota berhasil ditemukan.", reg, nil)
}

// GetDistrictsByRegency handles GET /api/v1/regencies/:code/districts
// @Summary      Daftar Kecamatan per Kabupaten/Kota
// @Description  Mengambil daftar kecamatan di bawah kabupaten/kota tertentu dengan pagination, search, dan sorting.
// @Tags         Districts
// @Produce      json
// @Param        code     path     string  true   "Kode Kabupaten/Kota Induk (contoh: 11.01)"
// @Param        page     query    int     false  "Nomor halaman (default: 1)"
// @Param        limit    query    int     false  "Jumlah data per halaman (default: 20, max: 100)"
// @Param        search   query    string  false  "Filter pencarian nama kecamatan"
// @Param        order    query    string  false  "Format pengurutan data: code:asc, code:desc, name:asc, name:desc"
// @Success      200      {object} response.ResponseEnvelope{data=[]domain.Region,meta=response.Meta}
// @Failure      400      {object} response.ResponseEnvelope
// @Failure      404      {object} response.ResponseEnvelope
// @Failure      500      {object} response.ResponseEnvelope
// @Router       /regencies/{code}/districts [get]
func (h *RegionReadHandler) GetDistrictsByRegency(c echo.Context) error {
	parentCode := c.Param("code")
	return h.handleChildrenQuery(c, parentCode, domain.LevelDistrict, "Data kecamatan berhasil diambil.")
}

// GetDistrictByCode handles GET /api/v1/districts/:code
// @Summary      Detail Kecamatan
// @Description  Mengambil informasi detail satu kecamatan berdasarkan kode wilayah unik (format: XX.YY.ZZ).
// @Tags         Districts
// @Produce      json
// @Param        code   path     string  true  "Kode Kecamatan (contoh: 11.01.01)"
// @Success      200    {object} response.ResponseEnvelope{data=domain.Region}
// @Failure      404    {object} response.ResponseEnvelope
// @Failure      500    {object} response.ResponseEnvelope
// @Router       /districts/{code} [get]
func (h *RegionReadHandler) GetDistrictByCode(c echo.Context) error {
	code := c.Param("code")
	reg, err := h.uc.GetByCode(c.Request().Context(), code)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "Kecamatan tidak ditemukan.", response.FieldError{Field: "code", Issue: err.Error()})
	}
	return response.Success(c, http.StatusOK, "Data kecamatan berhasil ditemukan.", reg, nil)
}

// GetVillagesByDistrict handles GET /api/v1/districts/:code/villages
// @Summary      Daftar Kelurahan/Desa per Kecamatan
// @Description  Mengambil daftar kelurahan dan desa di bawah kecamatan tertentu dengan pagination, search, dan sorting.
// @Tags         Villages
// @Produce      json
// @Param        code     path     string  true   "Kode Kecamatan Induk (contoh: 11.01.01)"
// @Param        page     query    int     false  "Nomor halaman (default: 1)"
// @Param        limit    query    int     false  "Jumlah data per halaman (default: 20, max: 100)"
// @Param        search   query    string  false  "Filter pencarian nama kelurahan/desa"
// @Param        order    query    string  false  "Format pengurutan data: code:asc, code:desc, name:asc, name:desc"
// @Success      200      {object} response.ResponseEnvelope{data=[]domain.Region,meta=response.Meta}
// @Failure      400      {object} response.ResponseEnvelope
// @Failure      404      {object} response.ResponseEnvelope
// @Failure      500      {object} response.ResponseEnvelope
// @Router       /districts/{code}/villages [get]
func (h *RegionReadHandler) GetVillagesByDistrict(c echo.Context) error {
	parentCode := c.Param("code")
	return h.handleChildrenQuery(c, parentCode, domain.LevelVillage, "Data kelurahan/desa berhasil diambil.")
}

// GetVillageByCode handles GET /api/v1/villages/:code
// @Summary      Detail Kelurahan/Desa
// @Description  Mengambil informasi detail satu kelurahan/desa berdasarkan kode wilayah unik (format: XX.YY.ZZ.WWWW).
// @Tags         Villages
// @Produce      json
// @Param        code   path     string  true  "Kode Kelurahan/Desa (contoh: 11.01.01.2001)"
// @Success      200    {object} response.ResponseEnvelope{data=domain.Region}
// @Failure      404    {object} response.ResponseEnvelope
// @Failure      500    {object} response.ResponseEnvelope
// @Router       /villages/{code} [get]
func (h *RegionReadHandler) GetVillageByCode(c echo.Context) error {
	code := c.Param("code")
	reg, err := h.uc.GetByCode(c.Request().Context(), code)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "Kelurahan/desa tidak ditemukan.", response.FieldError{Field: "code", Issue: err.Error()})
	}
	return response.Success(c, http.StatusOK, "Data kelurahan/desa berhasil ditemukan.", reg, nil)
}

// GetPostalCodeByVillage handles GET /api/v1/villages/:code/postal-code
// @Summary      Kode Pos Berdasarkan Kelurahan/Desa
// @Description  Mengambil data kode pos spesifik yang ditautkan ke satu kelurahan/desa berdasarkan kode wilayah kelurahan.
// @Tags         Villages
// @Produce      json
// @Param        code   path     string  true  "Kode Kelurahan/Desa (contoh: 11.01.01.2001)"
// @Success      200    {object} response.ResponseEnvelope{data=domain.VillagePostalCodeResponse}
// @Failure      404    {object} response.ResponseEnvelope
// @Failure      500    {object} response.ResponseEnvelope
// @Router       /villages/{code}/postal-code [get]
func (h *RegionReadHandler) GetPostalCodeByVillage(c echo.Context) error {
	code := c.Param("code")
	res, err := h.uc.GetPostalCodeByVillageCode(c.Request().Context(), code)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "Kelurahan/desa tidak ditemukan.", response.FieldError{Field: "code", Issue: err.Error()})
	}
	msg := "Kode pos kelurahan berhasil ditemukan."
	if res.PostalCode == nil {
		msg = "Data kelurahan ditemukan namun kode pos belum terdaftar."
	}
	return response.Success(c, http.StatusOK, msg, res, nil)
}

// GetPostalCodesByVillage handles GET /api/v1/villages/:code/postal-codes
// @Summary      Daftar Kode Pos per Kelurahan/Desa
// @Description  Mengambil daftar kode pos 5 digit yang terdaftar di bawah kelurahan/desa tertentu.
// @Tags         Postal Codes
// @Produce      json
// @Param        code     path     string  true   "Kode Kelurahan/Desa Induk (contoh: 11.01.01.2001)"
// @Param        page     query    int     false  "Nomor halaman (default: 1)"
// @Param        limit    query    int     false  "Jumlah data per halaman (default: 20, max: 100)"
// @Param        search   query    string  false  "Filter pencarian nomor kode pos"
// @Param        order    query    string  false  "Format pengurutan data: code:asc, code:desc, name:asc, name:desc"
// @Success      200      {object} response.ResponseEnvelope{data=[]domain.Region,meta=response.Meta}
// @Failure      400      {object} response.ResponseEnvelope
// @Failure      404      {object} response.ResponseEnvelope
// @Failure      500      {object} response.ResponseEnvelope
// @Router       /villages/{code}/postal-codes [get]
func (h *RegionReadHandler) GetPostalCodesByVillage(c echo.Context) error {
	parentCode := c.Param("code")
	return h.handleChildrenQuery(c, parentCode, domain.LevelPostalCode, "Data kode pos berhasil diambil.")
}

// GetPostalHierarchy handles GET /api/v1/postal-codes/:postal_code
// @Summary      Hierarki Wilayah Lengkap dari Kode Pos
// @Description  Reverse lookup hierarki lengkap (Desa, Kecamatan, Kabupaten, Provinsi) berdasarkan kode pos 5 digit.
// @Tags         Postal Codes
// @Produce      json
// @Param        postal_code  path     string  true  "Nomor Kode Pos 5 Digit (contoh: 23762, 10110)"
// @Success      200          {object} response.ResponseEnvelope{data=domain.PostalHierarchy}
// @Failure      404          {object} response.ResponseEnvelope
// @Failure      500          {object} response.ResponseEnvelope
// @Router       /postal-codes/{postal_code} [get]
func (h *RegionReadHandler) GetPostalHierarchy(c echo.Context) error {
	postalCode := c.Param("postal_code")
	hierarchy, err := h.uc.GetPostalHierarchy(c.Request().Context(), postalCode)
	if err != nil {
		return response.Error(c, http.StatusNotFound, "Data kode pos tidak ditemukan.", response.FieldError{Field: "postal_code", Issue: err.Error()})
	}
	return response.Success(c, http.StatusOK, "Hierarki wilayah berdasarkan kode pos berhasil diambil.", hierarchy, nil)
}

func (h *RegionReadHandler) handleChildrenQuery(c echo.Context, parentCode string, targetLevel int16, msg string) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	search := c.QueryParam("search")

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 1000 {
		limit = 20
	}

	orderBy, orderDir, err := sanitizer.ParseOrderParam(
		c.QueryParam("order"),
		c.QueryParam("order_by"),
		c.QueryParam("order_dir"),
	)
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "Format payload permintaan tidak valid.", response.FieldError{
			Field: "order",
			Issue: err.Error(),
		})
	}

	items, total, err := h.uc.GetChildrenByParent(c.Request().Context(), parentCode, targetLevel, domain.FilterParams{
		Search:   search,
		Page:     page,
		Limit:    limit,
		OrderBy:  orderBy,
		OrderDir: orderDir,
	})
	if err != nil {
		return response.Error(c, http.StatusNotFound, "Gagal mengambil data wilayah dari basis data.", response.FieldError{Field: "parent_code", Issue: err.Error()})
	}

	return response.SuccessPaginated(c, msg, items, page, limit, total)
}
