package usecase

import (
	"strings"
)

// ValidateJurisdiction memastikan targetCode berada di bawah hierarki userScope
func ValidateJurisdiction(userScope, targetCode string) bool {
	if userScope == "*" || userScope == "" {
		return true // Superadmin / Cakupan Nasional
	}
	// Target code harus diawali dengan userScope dan diikuti tanda titik (sub-wilayah)
	// atau persis sama dengan kode wilayah yurisdiksi tersebut.
	if targetCode == userScope {
		return true
	}
	return strings.HasPrefix(targetCode, userScope+".")
}
