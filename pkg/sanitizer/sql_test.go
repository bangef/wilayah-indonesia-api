package sanitizer_test

import (
	"testing"

	"be_region_indonesia/pkg/sanitizer"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeLikePattern(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Normal text", "Aceh", "Aceh"},
		{"Text with spaces", "  Jakarta Barat  ", "Jakarta Barat"},
		{"Percent wildcard", "100%", "100\\%"},
		{"Underscore wildcard", "user_admin", "user\\_admin"},
		{"Backslash and wildcards", "path\\to_%", "path\\\\to\\_\\%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizer.SanitizeLikePattern(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatPrefix(t *testing.T) {
	assert.Equal(t, "11.%", sanitizer.FormatPrefix("11"))
	assert.Equal(t, "11.01.%", sanitizer.FormatPrefix("  11.01  "))
}
