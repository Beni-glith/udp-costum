# UDP Custom Lite

Aplikasi ringan untuk **UDP tunneling only** (tanpa HTTP/WS/SSH), terdiri dari:
- `udpclt` (client)
- `udpsrv` (server)

## Format config client

```txt
<serverHost>:<udpPortSpec>@<token>:<localPort>
```

Contoh:

```txt
min.xhmt.my.id:1-65535@Trial25171:1
```

Aturan parsing:
- Format dasarnya kompatibel gaya `ip:port@user:pass` dengan arti final: `<serverHost>:<udpPortSpec>@<token>:<localPort>`
- `udpPortSpec = 1-65535` => `ANY_UDP_PORT=true`
- `udpPortSpec = angka` => hanya port itu yang diizinkan
- `udpPortSpec = rentang` (contoh `54-65535`) => izinkan hanya port dalam rentang itu
- `token` dipakai untuk HMAC-SHA256, dan boleh mengandung `:` (contoh `user:pass`)
- `localPort` adalah UDP listen pada `127.0.0.1:<localPort>`

## Build

```bash
go build -o udpclt ./cmd/udpclt
go build -o udpsrv ./cmd/udpsrv
```

## Menjalankan

Server:

```bash
./udpsrv --listen ":9000" --token "Trial25171" --dst-ip "8.8.8.8"
```

Client:

```bash
./udpclt \
  --config "min.xhmt.my.id:1-65535@Trial25171:5300" \
  --dst "8.8.8.8:53" \
  --server-port 9000
```

## End-to-end lokal (contoh)

1. Jalankan echo UDP sederhana:
   ```bash
   ncat -u -l 127.0.0.1 9999 -k -c 'xargs -n1 echo'
   ```
2. Jalankan server:
   ```bash
   ./udpsrv --listen ":9000" --token "demo" --dst-ip "127.0.0.1"
   ```
3. Jalankan client:
   ```bash
   ./udpclt --config "127.0.0.1:9999@demo:5300" --dst "127.0.0.1:9999" --server-port 9000
   ```
4. Kirim datagram ke local client port:
   ```bash
   echo ping | nc -u -w1 127.0.0.1 5300
   ```


## Android App (Client)

Project Android tersedia di folder `android/`.

Langkah cepat:
1. Buka folder `android/` di Android Studio.
2. Build dan run ke device Android.
3. Isi field:
   - `config`: contoh `min.xhmt.my.id:54-65535@Trial25171:5300`
   - `dst`: contoh `8.8.8.8:53`
   - `server tcp port`: contoh `9000`
4. Tap **Start** untuk mulai tunnel, aplikasi listen UDP di `127.0.0.1:<localPort>` (sesuai config).

Catatan:
- Android app ini fokus sebagai **client UDP tunnel**.
- Server tetap dijalankan di host/server menggunakan binary `udpsrv`.

## Fitur protokol
- Framing biner: magic `UDPC`, version `1`, flags, session_id, dst_port, payload_len
- HMAC-SHA256 (`32` byte) atas `header+body`
- Drop frame invalid (magic/version/len/hmac)
- Max payload default `1200` bytes (dapat diubah via flag)
- Keepalive default `15s`
- Reconnect otomatis client saat TCP putus
- Rate limit koneksi (`--rate-pps`, `--rate-bps`)
- UDP timeout server default `3s`
- Logging ringkas terstruktur

## Test

```bash
go test ./...
```
