package sanitizer_test

import (
	"testing"

	"be_region_indonesia/pkg/sanitizer"
	"github.com/stretchr/testify/assert"
)

func TestParseOrderParam(t *testing.T) {
	tests := []struct {
		name         string
		order        string
		orderBy      string
		orderDir     string
		expectedCol  string
		expectedDir  string
		expectError  bool
	}{
		{
			name:        "Default empty",
			order:       "",
			expectedCol: "code",
			expectedDir: "ASC",
		},
		{
			name:        "Shorthand desc",
			order:       "desc",
			expectedCol: "code",
			expectedDir: "DESC",
		},
		{
			name:        "Valid code:asc",
			order:       "code:asc",
			expectedCol: "code",
			expectedDir: "ASC",
		},
		{
			name:        "Valid name:desc",
			order:       "name:desc",
			expectedCol: "name",
			expectedDir: "DESC",
		},
		{
			name:        "Separate order_by and order_dir",
			orderBy:     "name",
			orderDir:    "desc",
			expectedCol: "name",
			expectedDir: "DESC",
		},
		{
			name:        "Invalid column",
			order:       "id:asc",
			expectError: true,
		},
		{
			name:        "Invalid direction",
			order:       "code:diagonal",
			expectError: true,
		},
		{
			name:        "SQL Injection attempt in order",
			order:       "code;DROP TABLE regions--",
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			col, dir, err := sanitizer.ParseOrderParam(tc.order, tc.orderBy, tc.orderDir)
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedCol, col)
				assert.Equal(t, tc.expectedDir, dir)
			}
		})
	}
}
