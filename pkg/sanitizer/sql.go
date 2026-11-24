package sanitizer

import "strings"

// SanitizeLikePattern membersihkan karakter wildcard SQL (% dan _) dari input pengguna
// untuk mencegah uncontrolled expansion atau DoS full-table scan pada klausul LIKE.
func SanitizeLikePattern(input string) string {
	escaped := strings.ReplaceAll(input, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "%", "\\%")
	escaped = strings.ReplaceAll(escaped, "_", "\\_")
	return strings.TrimSpace(escaped)
}

// FormatPrefix menghasilkan pola pencarian prefix yang aman
// Contoh: parentCode "11" -> "11.%"
func FormatPrefix(parentCode string) string {
	clean := SanitizeLikePattern(parentCode)
	return clean + ".%"
}
