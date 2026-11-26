# Standalone RESTful API Reference Specification
## Indonesian Regional Administrative Service (`be_region_indonesia`)
### Decoupled Documentation (Zero Runtime API Overhead)

| Attribute | Detail |
| :--- | :--- |
| **API Name** | Backend Regional Indonesia RESTful Service |
| **Protocol** | HTTP/1.1 (JSON over HTTPS) |
| **Base URL (Local)** | `http://127.0.0.1:8080/api/v1` |
| **Documentation Policy**| **Hybrid Architecture**: Dokumentasi statis lengkap tersedia di dokumen ini, serta dapat dieksplorasi secara visual dan interaktif melalui antarmuka Swagger UI di `/swagger/index.html` (dapat diaktifkan/dinonaktifkan melalui variabel `SWAGGER_ENABLED`). |
| **Auth Scheme** | Bearer Token (JWT) untuk endpoint mutasi (CRUD). Endpoint read bersifat publik. |
| **Last Updated** | 2026-11-24 |

---

## 1. Authentication & Security Headers

### 1.1 Endpoint Login Pengguna Admin (`POST /api/v1/auth/login`)
Melakukan autentikasi kredensial pengguna statis untuk mendapatkan JSON Web Token (JWT) yang digunakan pada endpoint mutasi.
- **URL:** `/api/v1/auth/login`
- **Method:** `POST`
- **Rate Limit:** Maksimal 10 requests/menit per IP Client (proteksi anti-brute force).
- **Kredensial Default Pengujian:**
  - `username`: `admin`
  - `password`: `AdminRegional2026!`
- **Request Body:**
```json
{
  "username": "admin",
  "password": "AdminRegional2026!"
}
```
- **Response `200 OK`:**
```json
{
  "success": true,
  "message": "Otentikasi berhasil. Sesi login telah dibuat.",
  "data": {
    "token_type": "Bearer",
    "access_token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "expires_in_seconds": 900,
    "user": {
      "id": "usr_admin_001",
      "username": "admin",
      "role": "ADMIN"
    }
  }
}
```

### 1.2 Public Endpoints
Endpoint pencarian dan pembacaan data (`GET`) bersifat publik dan tidak memerlukan token autentikasi, namun dilindungi oleh:
- **Rate Limit:** Maksimal 120 requests/menit per IP Client.
- **CORS:** Dibatasi hanya untuk origin resmi.

### 1.3 Protected Endpoints (CRUD)
Endpoint penambahan, perubahan, dan penghapusan data (`POST`, `PUT`, `DELETE`) mewajibkan header:
```http
Authorization: Bearer <JWT_ACCESS_TOKEN>
Content-Type: application/json
```

---

## 2. Standard Response Envelope

### 2.1 Success Response (`200 OK`, `201 Created`)
```json
{
  "success": true,
  "message": "Operasi berhasil diselesaikan",
  "data": {},
  "meta": {
    "page": 1,
    "limit": 20,
    "total_records": 100,
    "total_pages": 5
  }
}
```

### 2.2 Error Response (`400`, `401`, `403`, `404`, `409`, `422`, `429`, `500`)
```json
{
  "success": false,
  "message": "Format payload permintaan tidak valid / Data wilayah tidak ditemukan",
  "errors": [
    {
      "field": "code",
      "issue": "Field 'code' wajib diisi."
    }
  ]
}
```

---

## 3. Public Read Endpoints (Cascading Queries)

### 3.1 Daftar Provinsi (Level 1)
- **URL:** `GET /api/v1/provinces`
- **Query Params:**
  - `search` (string, optional): Filter nama provinsi (case-insensitive fuzzy search).
  - `page` (int, default: 1): Nomor halaman.
  - `limit` (int, default: 20): Jumlah data per halaman.
  - `order` (string, default: `code:asc`): Format `<kolom>:<arah>` (pilihan: `code:asc`, `code:desc`, `name:asc`, `name:desc`).
- **Example cURL:**
```bash
curl -X GET "http://127.0.0.1:8080/api/v1/provinces?search=aceh&page=1&limit=10&order=name:asc"
```
- **Response Sample:**
```json
{
  "success": true,
  "message": "Data provinsi berhasil diambil.",
  "data": [
    {
      "id": 1,
      "code": "11",
      "name": "ACEH",
      "level": 1,
      "type": "PROVINSI"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 10,
    "total_records": 1,
    "total_pages": 1
  }
}
```

---

### 3.2 Detail Satu Provinsi
- **URL:** `GET /api/v1/provinces/:code`
- **Example cURL:**
```bash
curl -X GET "http://127.0.0.1:8080/api/v1/provinces/11"
```

---

### 3.3 Daftar Kabupaten/Kota Berdasarkan Provinsi (Level 2)
- **URL:** `GET /api/v1/provinces/:code/regencies`
- **Path Param:** `:code` (Kode Provinsi, misal `11`)
- **Query Params:** `search`, `page`, `limit`, `order` (contoh: `name:desc`, default: `code:asc`)
- **SQL Execution:** `SELECT * FROM regions WHERE code LIKE '11.%' AND level = 2 ORDER BY name DESC`
- **Example cURL:**
```bash
curl -X GET "http://127.0.0.1:8080/api/v1/provinces/11/regencies?order=name:desc"
```
- **Response Sample:**
```json
{
  "success": true,
  "message": "Data kabupaten/kota berhasil diambil.",
  "data": [
    {
      "id": 15,
      "code": "11.11",
      "name": "KABUPATEN BIREUEN",
      "level": 2,
      "type": "KABUPATEN"
    },
    {
      "id": 16,
      "code": "11.71",
      "name": "KOTA BANDA ACEH",
      "level": 2,
      "type": "KOTA"
    }
  ],
  "meta": {
    "page": 1,
    "limit": 20,
    "total_records": 23,
    "total_pages": 2
  }
}
```

---

### 3.4 Daftar Kecamatan Berdasarkan Kabupaten/Kota (Level 3)
- **URL:** `GET /api/v1/regencies/:code/districts`
- **Path Param:** `:code` (Kode Kabupaten, misal `11.11`)
- **SQL Execution:** `SELECT * FROM regions WHERE code LIKE '11.11.%' AND level = 3`
- **Example cURL:**
```bash
curl -X GET "http://127.0.0.1:8080/api/v1/regencies/11.11/districts"
```

---

### 3.5 Daftar Kelurahan/Desa Berdasarkan Kecamatan (Level 4)
- **URL:** `GET /api/v1/districts/:code/villages`
- **Path Param:** `:code` (Kode Kecamatan, misal `11.11.01`)
- **SQL Execution:** `SELECT * FROM regions WHERE code LIKE '11.11.01.%' AND level = 4`
- **Example cURL:**
```bash
curl -X GET "http://127.0.0.1:8080/api/v1/districts/11.11.01/villages"
```

---

### 3.6 Daftar Kode Pos Berdasarkan Kelurahan (Level 5)
- **URL:** `GET /api/v1/villages/:code/postal-codes`
- **Path Param:** `:code` (Kode Kelurahan, misal `11.11.01.1001`)
- **SQL Execution:** `SELECT * FROM regions WHERE code LIKE '11.11.01.1001.%' AND level = 5`
- **Example cURL:**
```bash
curl -X GET "http://127.0.0.1:8080/api/v1/villages/11.11.01.1001/postal-codes"
```

---

### 3.7 Reverse Lookup Hierarki Berdasarkan Nilai Kode Pos
- **URL:** `GET /api/v1/postal-codes/:postal_code`
- **Path Param:** `:postal_code` (Nilai 5 digit kode pos, misal `24264`)
- **Fungsi:** Mengambil data hierarki lengkap (desa, kecamatan, kabupaten, dan provinsi) dalam 1 kali kueri.
- **Example cURL:**
```bash
curl -X GET "http://127.0.0.1:8080/api/v1/postal-codes/24264"
```
- **Response Sample:**
```json
{
  "success": true,
  "message": "Hierarki wilayah berdasarkan kode pos berhasil diambil.",
  "data": {
    "postal_code": "24264",
    "village": {
      "code": "11.11.01.1001",
      "name": "KEUDE SAMALANGA",
      "type": "GAMPONG"
    },
    "district": {
      "code": "11.11.01",
      "name": "SAMALANGA"
    },
    "regency": {
      "code": "11.11",
      "name": "KABUPATEN BIREUEN",
      "type": "KABUPATEN"
    },
    "province": {
      "code": "11",
      "name": "ACEH"
    }
  }
}
```

---

### 3.8 Kode Pos Spesifik Berdasarkan Kode Kelurahan/Desa
- **URL:** `GET /api/v1/villages/:code/postal-code`
- **Path Param:** `:code` (Kode Kelurahan/Desa 10 digit, misal `11.01.01.2001`)
- **Fungsi:** Mengambil kode pos yang ditautkan langsung pada kelurahan/desa.
- **Example cURL:**
```bash
curl -X GET "http://127.0.0.1:8080/api/v1/villages/11.01.01.2001/postal-code"
```
- **Response Sample:**
```json
{
  "success": true,
  "message": "Kode pos kelurahan berhasil ditemukan.",
  "data": {
    "village_code": "11.01.01.2001",
    "village_name": "KEUDE BAKONGAN",
    "postal_code": "23762"
  }
}
```

---

## 4. Protected CRUD Endpoints (Write Mutations)

### 4.1 Tambah Provinsi (Level 1)
- **URL:** `POST /api/v1/provinces`
- **Headers:** `Authorization: Bearer <token>`, `Content-Type: application/json`
- **Request Body:**
```json
{
  "code": "11",
  "name": "ACEH",
  "type": "PROVINSI"
}
```
- **Response Status:** `201 Created`

---

### 4.2 Tambah Kabupaten/Kota (Level 2)
- **URL:** `POST /api/v1/provinces/:code/regencies`
- **Path Param:** `:code` (Kode Parent Provinsi, misal `11`)
- **Validation Rule:** Field `code` **wajib** diawali dengan `:code.` (contoh: `11.11`).
- **Request Body:**
```json
{
  "code": "11.11",
  "name": "KABUPATEN BIREUEN",
  "type": "KABUPATEN"
}
```
- **Response Status:** `201 Created`

---

### 4.3 Tambah Kecamatan (Level 3)
- **URL:** `POST /api/v1/regencies/:code/districts`
- **Path Param:** `:code` (Kode Parent Kabupaten, misal `11.11`)
- **Validation Rule:** Field `code` **wajib** diawali dengan `:code.` (contoh: `11.11.01`).
- **Request Body:**
```json
{
  "code": "11.11.01",
  "name": "SAMALANGA",
  "type": "KECAMATAN"
}
```
- **Response Status:** `201 Created`

---

### 4.4 Tambah Kelurahan/Desa (Level 4)
- **URL:** `POST /api/v1/districts/:code/villages`
- **Path Param:** `:code` (Kode Parent Kecamatan, misal `11.11.01`)
- **Validation Rule:** Field `code` **wajib** diawali dengan `:code.` (contoh: `11.11.01.1001`).
- **Request Body:**
```json
{
  "code": "11.11.01.1001",
  "name": "KEUDE SAMALANGA",
  "type": "GAMPONG"
}
```
- **Response Status:** `201 Created`

---

### 4.5 Tambah Kode Pos (Level 5)
- **URL:** `POST /api/v1/villages/:code/postal-codes`
- **Path Param:** `:code` (Kode Parent Kelurahan, misal `11.11.01.1001`)
- **Request Body:**
```json
{
  "postal_code": "24264"
}
```
- **Business Logic:** Sistem otomatis membentuk `code = "11.11.01.1001.24264"`, `name = "24264"`, `level = 5`, `type = "KODE_POS"`.
- **Response Status:** `201 Created`

---

### 4.6 Hapus Wilayah (Soft Delete)
- **URL:** `DELETE /api/v1/regions/:code`
- **Path Param:** `:code`
- **Business Logic:** Sistem memvalidasi apakah ada child aktif (`WHERE code LIKE :code || '.%' AND deleted_at IS NULL`). Jika ada child aktif, penghapusan ditolak dengan status `409 Conflict`. Jika bersih, status menjadi soft delete dan cache terkait dihapus.
- **Response Status:** `200 OK`

---

## 5. Observabilitas, Logging Terstruktur & VictoriaLogs

Aplikasi memancarkan log terstruktur berformat JSON langsung ke `os.Stdout` menggunakan pustaka bawaan Go `log/slog`. Log ditangkap oleh `systemd-journald`, diproses oleh **Vector**, dan dikirimkan ke **VictoriaLogs** dengan tata kelola kardinalitas rendah.

### 5.1 Format Log HTTP Request
Setiap request HTTP dicatat dalam format JSON satu baris (*newline-delimited JSON*):
```json
{
  "time": "2026-11-25T10:30:15.123456789+07:00",
  "level": "INFO",
  "msg": "http request completed",
  "service": "be_region_indonesia",
  "env": "prod",
  "method": "GET",
  "path": "/api/v1/villages/:code/postal-code",
  "uri": "/api/v1/villages/11.01.01.2001/postal-code",
  "status": 200,
  "latency_ms": 1.45,
  "remote_ip": "127.0.0.1",
  "request_id": "c1f7a0b3-9e6b-4e12-b9cf-2a3b4c5d6e7f"
}
```

### 5.2 Katalog Kueri LogsQL pada `vmui` (Port 9428)
Antarmuka web VictoriaLogs UI dapat diakses pada `http://<IP-VPS>:9428/select/vmui/`:

- **Melihat seluruh log aplikasi:**
  ```logsql
  service:"be_region_indonesia" | fields _time, level, status, method, path, latency_ms, request_id, msg
  ```
- **Melacak seluruh jejak request berdasarkan Request ID:**
  ```logsql
  request_id:"c1f7a0b3-9e6b-4e12-b9cf-2a3b4c5d6e7f"
  ```
- **Menemukan log error server (HTTP 5xx):**
  ```logsql
  service:"be_region_indonesia" AND status:>=500
  ```
- **Menemukan request lambat (*slow request* > 200 ms):**
  ```logsql
  service:"be_region_indonesia" AND latency_ms:>200
  ```
- **Distribusi status HTTP berdasarkan pola rute:**
  ```logsql
  service:"be_region_indonesia" | stats by (path, status) count()
  ```

