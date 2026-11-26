package migrator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type MigrationFile struct {
	Version  string // e.g. "000001"
	Name     string // e.g. "000001_create_regions_table"
	UpPath   string // path to .up.sql
	DownPath string // path to .down.sql
}

type AppliedMigration struct {
	ID        int
	Version   string
	Name      string
	Batch     int
	AppliedAt string
}

type MigrationStatus struct {
	Version   string
	Name      string
	Batch     *int
	AppliedAt *string
	Status    string // "APPLIED" or "PENDING"
}

var migrationRegex = regexp.MustCompile(`^([0-9]{6})_([a-zA-Z0-9_-]+)\.(up|down)\.sql$`)

// ScanMigrations memindai direktori migrations dan memvalidasi kelengkapan berkas
func ScanMigrations(dir string) ([]MigrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("gagal membaca direktori migrasi %s: %w", dir, err)
	}

	upFiles := make(map[string]string)
	downFiles := make(map[string]string)
	names := make(map[string]string)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matches := migrationRegex.FindStringSubmatch(entry.Name())
		if len(matches) != 4 {
			continue // Abaikan file yang tidak sesuai pola (regex has 3 capture groups: version, desc, up/down -> total len 4)
		}

		version := matches[1]
		desc := matches[2]
		fullName := fmt.Sprintf("%s_%s", version, desc)
		names[version] = fullName

		if strings.HasSuffix(entry.Name(), ".up.sql") {
			upFiles[version] = filepath.Join(dir, entry.Name())
		} else if strings.HasSuffix(entry.Name(), ".down.sql") {
			downFiles[version] = filepath.Join(dir, entry.Name())
		}
	}

	var result []MigrationFile
	for version, fullName := range names {
		upPath, hasUp := upFiles[version]
		downPath, hasDown := downFiles[version]

		if !hasUp {
			return nil, fmt.Errorf("migrasi %s memiliki berkas .down.sql namun kehilangan berkas .up.sql", fullName)
		}
		if !hasDown {
			return nil, fmt.Errorf("migrasi %s memiliki berkas .up.sql namun kehilangan berkas .down.sql", fullName)
		}

		result = append(result, MigrationFile{
			Version:  version,
			Name:     fullName,
			UpPath:   upPath,
			DownPath: downPath,
		})
	}

	// Urutkan ascending berdasarkan versi
	sort.Slice(result, func(i, j int) bool {
		return result[i].Version < result[j].Version
	})

	return result, nil
}
