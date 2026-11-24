# [wilayah-indonesia-api](https://github.com/bangef/wilayah-indonesia-api)

> **Public Repository**  
> API backend Go berkinerja tinggi untuk 83.000+ data wilayah Indonesia dengan arsitektur single-table, Redis cache, bulk CSV importer kode pos, dan dokumentasi Swagger interaktif.

[![Go Version](https://img.shields.io/badge/Go-1.21%2B-blue.svg)](https://golang.org)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15%2B-336791.svg)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-7%2B-DC382D.svg)](https://redis.io/)
[![Echo Framework](https://img.shields.io/badge/Echo-v4-00ADD8.svg)](https://echo.labstack.com/)
[![Swagger](https://img.shields.io/badge/Swagger-OpenAPI%202.0-85EA2D.svg)](http://localhost:8080/swagger/index.html)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

Backend RESTful API berkinerja tinggi untuk mengelola dan menyajikan data hierarki wilayah administratif Republik Indonesia (Provinsi, Kabupaten/Kota, Kecamatan, hingga Kelurahan/Desa) serta pemetaan kode pos. Dibangun menggunakan arsitektur bersih (*Clean Architecture*), skema tabel tunggal berindeks tinggi (*Single Table Design*), caching Redis bertingkat anti-*cache stampede* (Singleflight), proteksi keamanan JWT + Sliding Window Rate Limiting, serta utilitas CLI untuk impor massal (*bulk import*) kode pos dari berkas CSV.

---

## 📑 Daftar Isi

- [Fitur Utama](#-fitur-utama)
- [Arsitektur & Tech Stack](#-arsitektur--tech-stack)
- [Struktur Hierarki Wilayah](#-struktur-hierarki-wilayah)
- [Struktur Direktori Proyek](#-struktur-direktori-proyek)
- [Prasyarat Sistem](#-prasyarat-sistem)
- [Panduan Instalasi & Menjalankan](#-panduan-instalasi--menjalankan)
- [Dokumentasi API & Endpoint](#-dokumentasi-api--endpoint)
- [Utilitas CLI Importer Kode Pos (CSV)](#-utilitas-cli-importer-kode-pos-csv)
- [Testing & Quality Assurance](#-testing--quality-assurance)
- [Deployment Systemd](#-deployment-systemd)
- [Lisensi](#-lisensi)

---

## 🚀 Fitur Utama

1. **83.000+ Data Wilayah Administratif Indonesia:**
   - Menyajikan 38 Provinsi, 514 Kabupaten/Kota, 7.200+ Kecamatan, dan 83.000+ Kelurahan/Desa.
   - Paging, filtering, dan dynamic ordering (`?order=name:asc`, `?order=code:desc`) pada semua endpoint daftar.
2. **Kueri Kode Pos Fleksibel:**
   - Pengambilan kode pos berdasarkan kode kelurahan (`GET /api/v1/villages/:code/postal-code`).
   - Reverse lookup hierarki lengkap dari kode pos (`GET /api/v1/postal-codes/:code`).
3. **High-Performance Caching & Stampede Protection:**
   - Caching terdistribusi Redis dengan TTL 7 hari.
   - Proteksi `golang.org/x/sync/singleflight` untuk meredam lonjakan beban (*dogpiling/cache stampede*).
   - Latensi baca $p95 < 10\text{ ms}$ pada pengujian beban tinggi.
4. **Keamanan & Rate Limiting:**
   - Kredensial login admin terproteksi constant-time comparison (anti-*timing attacks*).
   - Akses mutasi data (POST/PUT/DELETE) dilindungi JWT Authentication dengan revocation blacklist.
   - Rate limiting Redis Sliding Window independen untuk public read, write admin, search, dan login.
5. **Interactive Swagger Documentation:**
   - Swagger UI interaktif langsung di `/swagger/index.html` (dapat diaktifkan/dinonaktifkan via environment variable).
6. **Bulk Postal Code Importer:**
   - Utilitas CLI mandiri untuk mengimpor atau memperbarui puluhan ribu kode pos langsung dari berkas CSV menggunakan teknik PostgreSQL `UNNEST` berkecepatan tinggi.

---

## 🛠 Arsitektur & Tech Stack

- **Bahasa Pemrograman:** Go 1.21+
- **HTTP Framework:** [Echo v4](https://echo.labstack.com/)
- **Database:** PostgreSQL 15+ (Native Linux Service)
- **In-Memory Cache:** Redis 7+ (Native Linux Service)
- **Dokumentasi API:** Swagger / OpenAPI via [swaggo/swag](https://github.com/swaggo/swag)
- **Arsitektur:** Clean Architecture (Domain, Repository, Usecase, Delivery/HTTP Handler)

---

## 🏛 Struktur Hierarki Wilayah

Sistem menggunakan kode standar nasional berjenjang (dot-separated prefix):

| Level | Tingkat Wilayah | Format Kode | Contoh Kode | Contoh Nama |
| :---: | :--- | :--- | :--- | :--- |
| **1** | Provinsi | `XX` | `11` | ACEH |
| **2** | Kabupaten / Kota | `XX.XX` | `11.01` | KABUPATEN ACEH SELATAN |
| **3** | Kecamatan | `XX.XX.XX` | `11.01.01` | BAKONGAN |
| **4** | Kelurahan / Desa | `XX.XX.XX.XXXX` | `11.01.01.2001` | KEUDE BAKONGAN (Kode Pos: `23773`) |

---

## 📂 Struktur Direktori Proyek

```
be_region_indonesia/
├── bin/                       # Output biner hasil kompilasi
├── cmd/
│   ├── api/                   # Entry point server HTTP API
│   ├── postal_importer/       # Utilitas CLI bulk import kode pos CSV
│   └── seeder/                # Seeder database awal wilayah Indonesia (BPS)
├── config/                    # Manajemen konfigurasi environment (.env)
├── data/
│   └── postal_codes_template.csv # Template CSV kode pos
├── deploy/                    # Unit file systemd untuk deployment server
├── docs/
│   ├── API_DOCUMENTATION.md   # Spesifikasi lengkap API dalam Markdown
│   └── swagger/               # Berkas Swagger/OpenAPI (docs.go, swagger.json, swagger.yaml)
├── internal/
│   ├── delivery/http/         # Echo router, handlers, dan middleware
│   ├── domain/                # Entitas bisnis inti, DTO, dan contracts
│   ├── repository/            # PostgreSQL repository dan Redis cache repository
│   └── usecase/               # Business logic, validasi hierarki & orkestrasi cache
├── migrations/                # Berkas migrasi database SQL (Up & Down)
├── phases/                    # Dokumentasi phase-gate roadmap (Phase 0 - 13)
├── pkg/                       # Utilitas reusable (database, sanitizer, validator, response)
├── scripts/data/              # Dataset SQL wilayah BPS
├── tests/                     # Integration tests & load tests (k6)
├── .env.example               # Template environment configuration
└── Makefile                   # Automasi build, test, migrasi, dan seed
```

---

## ⚙️ Prasyarat Sistem

Pastikan layanan native berikut telah terinstal dan berjalan pada sistem:
1. **Go 1.21+** (`go version`)
2. **PostgreSQL 15+** (`pg_isready -h 127.0.0.1 -p 5432`)
3. **Redis 7+** (`redis-cli ping`)
4. **Make** & **curl**

---

## 🚦 Panduan Instalasi & Menjalankan

### 1. Salin Konfigurasi Environment
```bash
cp .env.example .env
```
Sesuaikan kredensial database PostgreSQL dan Redis Anda di dalam berkas `.env`.

### 2. Jalankan Migrasi Database
```bash
make migrate-up
```

### 3. Eksekusi Seeder Data Wilayah (Level 1-4)
```bash
make seed
```
> *Catatan: Proses seeding akan memuat 83.000+ data wilayah administratif ke dalam PostgreSQL dalam tempo ~2-5 detik menggunakan PostgreSQL COPY protocol.*

### 4. Jalankan Server API
```bash
make run
```
Aplikasi akan berjalan pada port `8080` (default) di: `http://localhost:8080`.

---

## 📖 Dokumentasi API & Endpoint

### Swagger UI Interaktif
Jika `SWAGGER_ENABLED=true` pada `.env`, buka browser pada:
👉 **[http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)**

### Ringkasan Endpoint Utama

#### 1. Otentikasi
* `POST /api/v1/auth/login` - Login admin untuk mendapatkan token JWT (Kredensial default: `admin` / `AdminRegional2026!`).

#### 2. Publik: Navigasi Hierarki Wilayah (Cached)
* `GET /api/v1/provinces` - Daftar seluruh provinsi.
* `GET /api/v1/provinces/:code/regencies` - Daftar kabupaten/kota di provinsi terkait.
* `GET /api/v1/regencies/:code/districts` - Daftar kecamatan di kabupaten terkait.
* `GET /api/v1/districts/:code/villages` - Daftar kelurahan/desa di kecamatan terkait.
* `GET /api/v1/regions/:code` - Detail spesifik wilayah berdasarkan kode unik.

#### 3. Publik: Kode Pos
* `GET /api/v1/villages/:code/postal-code` - Ambil kode pos berdasarkan kode kelurahan.
* `GET /api/v1/postal-codes/:code` - Ambil hierarki wilayah lengkap dari kode pos.

#### 4. Terproteksi (Bearer Token): Mutasi Data
* `PUT /api/v1/villages/:code/postal-code` - Perbarui kode pos kelurahan.
* `POST /api/v1/provinces` - Tambah provinsi baru.
* `POST /api/v1/provinces/:code/regencies` - Tambah kabupaten baru.
* `PUT /api/v1/regions/:code` - Perbarui nama/tipe wilayah.
* `DELETE /api/v1/regions/:code` - Hapus wilayah (*soft delete*, proteksi bila memiliki anak aktif).

> Untuk rincian payload request dan response lengkap, silakan lihat dokumen [docs/API_DOCUMENTATION.md](docs/API_DOCUMENTATION.md).

---

## 📦 Utilitas CLI Importer Kode Pos (CSV)

Proyek ini dilengkapi dengan CLI berkecepatan tinggi untuk memperbarui kolom `postal_code` secara massal dari berkas CSV.

Template CSV tersedia pada: [`data/postal_codes_template.csv`](data/postal_codes_template.csv). Format kolom:
```csv
code,postal_code
11.01.01.2001,23773
31.71.01.1001,10110
32.73.01.1001,40111
```

### Cara Menjalankan Importer:
```bash
# 1. Mode Simulasi (Dry-Run) tanpa mengubah database:
make import-postal-dry-run

# 2. Mode Simulasi pada file CSV kustom:
make import-postal-dry-run FILE=path/to/data.csv

# 3. Eksekusi Aktual (Commit ke PostgreSQL & Invalidate Cache):
make import-postal-code

# 4. Eksekusi Aktual pada file CSV kustom:
make import-postal-code FILE=path/to/data.csv
```

---

## 🧪 Testing & Quality Assurance

Proyek ini menerapkan cakupan pengujian komprehensif (Unit Test, Integration Test, Load Test, dan Data Race Detector):

```bash
# Menjalankan seluruh test suite dengan Go Race Detector
make test

# Menjalankan health check database dan redis
make check-all

# Mengompilasi binary siap produksi (stripped debug info)
make build
```

---

## 🚢 Deployment Systemd

Untuk lingkungan produksi Ubuntu/Debian Linux menggunakan systemd:

```bash
# 1. Kompilasi biner
make build

# 2. Pasang file service systemd
make install-service

# 3. Mulai service
make service-start

# 4. Periksa status service
make service-status
```

---

## 📄 Lisensi

Didistribusikan di bawah lisensi **MIT License**. Lihat file `LICENSE` untuk informasi lebih lanjut.
