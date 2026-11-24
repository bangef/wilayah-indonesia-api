import http from 'k6/http';
import { check, sleep } from 'k6';

// Konfigurasi skenario beban k6
export const options = {
  stages: [
    { duration: '15s', target: 50 },  // Ramp-up ke 50 VUs
    { duration: '30s', target: 200 }, // Sustained load 200 VUs
    { duration: '15s', target: 0 },   // Ramp-down
  ],
  thresholds: {
    // 95% request harus selesai di bawah 15ms
    http_req_duration: ['p(95)<15'],
    // Error rate harus 0%
    http_req_failed: ['rate==0'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://127.0.0.1:8080/api/v1';

export default function () {
  // Test 1: List Provinsi
  const resProvinces = http.get(`${BASE_URL}/provinces`);
  check(resProvinces, {
    'status is 200': (r) => r.status === 200,
    'has 38 provinces': (r) => r.json('meta.total_records') === 38,
  });

  // Test 2: Cascading Kabupaten di Provinsi Aceh (11)
  const resRegencies = http.get(`${BASE_URL}/provinces/11/regencies`);
  check(resRegencies, {
    'status is 200': (r) => r.status === 200,
    'has 23 regencies in Aceh': (r) => r.json('meta.total_records') === 23,
  });

  sleep(0.05);
}
