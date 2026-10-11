# PingBoard

Dashboard **uptime monitor** sederhana: tambahkan URL, lalu backend akan mengecek
statusnya secara berkala dan frontend menampilkan status, uptime, serta grafik
latensi secara live.

Dibangun dengan **Go** (backend) dan **React** (frontend). **Tanpa database**:
semua data disimpan di memori.

## Fitur

- Tambah dan hapus monitor (URL, nama, interval 10 detik sampai 1 jam)
- Pengecekan otomatis berkala, satu **goroutine** per monitor
- Tombol "Cek sekarang" untuk pengecekan manual
- Status ONLINE / OFFLINE, persentase uptime, latensi rata-rata, grafik latensi
- Riwayat 60 pengecekan terakhir per monitor
- Tema terang/gelap otomatis mengikuti sistem, responsif untuk mobile

## Tech stack

| Bagian | Teknologi |
|---|---|
| Backend | Go 1.22+ (hanya standard library: `net/http`, `sync`, `context`) |
| Frontend | React 19 + Vite |
| Deploy | Docker multi-stage (image akhir berbasis distroless) |

## Menjalankan (development)

Butuh Go 1.22+ dan Node 20+.

```bash
# terminal 1: backend di :8080
cd backend
go run .

# terminal 2: frontend di :5173 (otomatis proxy /api ke :8080)
cd frontend
npm install
npm run dev
```

Buka http://localhost:5173.

## Menjalankan dengan Docker

```bash
docker build -t pingboard .
docker run --rm -p 8080:8080 pingboard
```

Buka http://localhost:8080. Satu binary Go melayani API sekaligus file statis React.

## Test

```bash
cd backend
go test -race ./...
```

## API

| Method | Endpoint | Keterangan |
|---|---|---|
| GET | `/api/health` | Health check |
| GET | `/api/monitors` | Daftar semua monitor beserta statistik dan riwayat |
| POST | `/api/monitors` | Buat monitor: `{"name": "...", "url": "https://...", "intervalSec": 30}` |
| GET | `/api/monitors/{id}` | Detail satu monitor |
| POST | `/api/monitors/{id}/check` | Cek sekarang |
| DELETE | `/api/monitors/{id}` | Hapus monitor |

Contoh:

```bash
curl -X POST localhost:8080/api/monitors \
  -H 'Content-Type: application/json' \
  -d '{"name":"Go.dev","url":"https://go.dev","intervalSec":30}'
```

## Konfigurasi (environment variable)

| Variabel | Default | Keterangan |
|---|---|---|
| `PORT` | `8080` | Port server |
| `STATIC_DIR` | kosong | Folder hasil build frontend; jika diisi, server ikut melayani frontend |
| `SEED_DEMO` | `true` | Isi 3 monitor contoh saat start; set `false` untuk menonaktifkan |
| `ALLOW_PRIVATE` | `false` | Izinkan mengecek alamat privat/localhost (lihat catatan keamanan) |
| `CORS_ORIGIN` | kosong | Isi jika frontend di-host di domain berbeda |

## Keputusan desain

- **Satu goroutine per monitor**, dihentikan lewat `context` saat monitor dihapus
  atau server mati. Ini menunjukkan concurrency Go tanpa library tambahan.
- **Graceful shutdown**: server menunggu request selesai dan semua goroutine
  berhenti sebelum keluar.
- **State di memori** dilindungi `sync.RWMutex`; riwayat dibatasi 60 entri per
  monitor agar memori tidak membengkak. Test dijalankan dengan `-race`.
- **Pengecekan HTTP bisa diganti** (`CheckFunc`), sehingga logika store bisa
  di-test tanpa jaringan.
- **Batas dan validasi input**: maksimal 20 monitor, hanya `http`/`https`,
  interval 10 sampai 3600 detik, ukuran body dibatasi, field tak dikenal ditolak.

## Catatan keamanan

Aplikasi ini membuat server melakukan request ke URL pilihan pengguna, yang
berisiko **SSRF**. Karena itu alamat loopback, privat (10.x, 192.168.x, 172.16.x),
link-local (169.254.x, termasuk metadata cloud), dan CGNAT diblokir saat koneksi
dibuat, sehingga berlaku juga untuk redirect dan hasil DNS. Set `ALLOW_PRIVATE=true`
hanya untuk lingkungan lokal.

Aplikasi ini juga tidak memiliki autentikasi. Jika di-deploy publik, pertimbangkan
menambahkan login atau rate limiting.

## Struktur proyek

```
pingboard/
├── backend/
│   ├── main.go          # entry point, graceful shutdown
│   ├── monitor.go       # Store, goroutine per monitor, validasi
│   ├── checker.go       # HTTP checker + proteksi SSRF
│   ├── handlers.go      # REST API + serve static/SPA
│   ├── middleware.go    # logger, recover, CORS, security headers
│   └── monitor_test.go
├── frontend/
│   └── src/
│       ├── App.jsx      # state + polling
│       ├── api.js
│       └── components/  # MonitorCard, Sparkline, AddMonitorForm, ...
├── Dockerfile
└── README.md
```

## Ide pengembangan lanjutan

- Persistensi opsional (file JSON atau SQLite) agar data tidak hilang saat restart
- Notifikasi saat status berubah (webhook, email)
- Update real-time dengan Server-Sent Events menggantikan polling
- Autentikasi dan rate limiting
- Halaman detail dengan riwayat lebih panjang
