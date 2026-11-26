.PHONY: run test build db-check redis-check check-all seed migrate-up migrate-down migrate-rollback migrate-refresh migrate-reset migrate-version migrate-status swagger

run:
	go run cmd/api/main.go

test:
	go test -v -race ./...

build:
	mkdir -p bin
	go build -ldflags="-s -w" -o bin/api cmd/api/main.go

db-check:
	pg_isready -h 127.0.0.1 -p 5432

redis-check:
	redis-cli -h 127.0.0.1 -p 6379 ping

migrate-up:
	@go run cmd/migrator/main.go -action=up

migrate-rollback:
	@go run cmd/migrator/main.go -action=rollback -steps=$(or $(STEPS),1)

migrate-down: migrate-rollback

migrate-refresh:
	@go run cmd/migrator/main.go -action=refresh -seed=$(or $(SEED),false) -force=$(or $(FORCE),false)

migrate-reset:
	@go run cmd/migrator/main.go -action=reset -force=$(or $(FORCE),false)

migrate-version:
	@if [ -z "$(VERSION)" ]; then \
		echo "[ERROR] Parameter VERSION wajib diisi. Contoh: make migrate-version VERSION=000002"; \
		exit 1; \
	fi
	@go run cmd/migrator/main.go -action=version -target=$(VERSION)

migrate-status:
	@go run cmd/migrator/main.go -action=status

swagger:
	@which swag > /dev/null 2>&1 || (echo "Menginstal swag CLI..." && go install github.com/swaggo/swag/cmd/swag@latest)
	swag init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal
	@echo "Dokumentasi Swagger berhasil diperbarui di direktori docs/swagger/"

seed:
	go run cmd/seeder/main.go

import-postal-code:
	@if [ -z "$(FILE)" ]; then \
		echo "[INFO] Parameter FILE tidak dispesifikasikan, menggunakan template default: data/postal_codes_template.csv"; \
		go run cmd/postal_importer/main.go -file=data/postal_codes_template.csv; \
	else \
		echo "[INFO] Menjalankan import kode pos dari: $(FILE)"; \
		go run cmd/postal_importer/main.go -file=$(FILE); \
	fi

import-postal-dry-run:
	@if [ -z "$(FILE)" ]; then \
		echo "[INFO] Parameter FILE tidak dispesifikasikan, menjalankan dry-run pada: data/postal_codes_template.csv"; \
		go run cmd/postal_importer/main.go -file=data/postal_codes_template.csv -dry-run=true; \
	else \
		echo "[INFO] Menjalankan dry-run import kode pos dari: $(FILE)"; \
		go run cmd/postal_importer/main.go -file=$(FILE) -dry-run=true; \
	fi

install-service:
	@echo "Menginstal systemd unit file..."
	sudo cp deploy/be_region_indonesia.service /etc/systemd/system/
	sudo systemctl daemon-reload
	sudo systemctl enable be_region_indonesia
	@echo "Service be_region_indonesia berhasil diaktifkan. Gunakan 'make service-start' untuk memulai."

service-start:
	sudo systemctl start be_region_indonesia

service-stop:
	sudo systemctl stop be_region_indonesia

service-restart:
	sudo systemctl restart be_region_indonesia

service-status:
	sudo systemctl status be_region_indonesia

check-all: db-check redis-check test
