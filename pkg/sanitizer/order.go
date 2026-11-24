package sanitizer

import (
	"errors"
	"strings"
)

var (
	ErrInvalidOrderParam = errors.New("parameter 'order' tidak valid. Nilai yang diperbolehkan adalah 'code:asc', 'code:desc', 'name:asc', atau 'name:desc'")
)

// ParseOrderParam memvalidasi dan mengekstrak parameter pengurutan secara aman (whitelist)
func ParseOrderParam(rawOrder, rawOrderBy, rawOrderDir string) (orderBy, orderDir string, err error) {
	// 1. Nilai default
	orderBy = "code"
	orderDir = "ASC"

	// 2. Jika parameter `order` diisi dengan format <kolom>:<arah> atau shorthand <arah>
	if strings.TrimSpace(rawOrder) != "" {
		clean := strings.ToLower(strings.TrimSpace(rawOrder))
		if clean == "asc" || clean == "desc" {
			orderDir = strings.ToUpper(clean)
			return orderBy, orderDir, nil
		}

		parts := strings.Split(clean, ":")
		if len(parts) == 2 {
			col := strings.TrimSpace(parts[0])
			dir := strings.ToUpper(strings.TrimSpace(parts[1]))

			if (col != "code" && col != "name") || (dir != "ASC" && dir != "DESC") {
				return "", "", ErrInvalidOrderParam
			}
			return col, dir, nil
		}
		return "", "", ErrInvalidOrderParam
	}

	// 3. Fallback jika menggunakan query params terpisah: order_by & order_dir
	if strings.TrimSpace(rawOrderBy) != "" {
		col := strings.ToLower(strings.TrimSpace(rawOrderBy))
		if col != "code" && col != "name" {
			return "", "", ErrInvalidOrderParam
		}
		orderBy = col
	}

	if strings.TrimSpace(rawOrderDir) != "" {
		dir := strings.ToUpper(strings.TrimSpace(rawOrderDir))
		if dir != "ASC" && dir != "DESC" {
			return "", "", ErrInvalidOrderParam
		}
		orderDir = dir
	}

	return orderBy, orderDir, nil
}
