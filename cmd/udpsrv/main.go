package main

import (
	"flag"
	"log"
	"net"
	"sync"
	"time"

	"udp-custom-lite/internal/framing"
)

type rateLimiter struct {
	mu      sync.Mutex
	window  time.Time
	packets int
	bytes   int
	pps     int
	bps     int
}

func newRateLimiter(pps, bps int) *rateLimiter {
	return &rateLimiter{window: time.Now(), pps: pps, bps: bps}
}

func (r *rateLimiter) Allow(n int) bool {
	if r.pps <= 0 && r.bps <= 0 {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if now.Sub(r.window) >= time.Second {
		r.window = now
		r.packets = 0
		r.bytes = 0
	}
	if r.pps > 0 && r.packets+1 > r.pps {
		return false
	}
	if r.bps > 0 && r.bytes+n > r.bps {
		return false
	}
	r.packets++
	r.bytes += n
	return true
}

func main() {
	listen := flag.String("listen", ":9000", "TCP listen address")
	token := flag.String("token", "", "auth token")
	maxPayload := flag.Int("max-payload", framing.DefaultMaxPayload, "max payload bytes")
	udpTimeout := flag.Duration("udp-timeout", 3*time.Second, "upstream UDP timeout")
	dstIP := flag.String("dst-ip", "127.0.0.1", "default upstream destination IP")
	ppsLimit := flag.Int("rate-pps", 0, "packets/sec limit per conn (0 disable)")
	bpsLimit := flag.Int("rate-bps", 0, "bytes/sec limit per conn (0 disable)")
	flag.Parse()

	if *token == "" {
		log.Fatal("event=start status=fail err=missing token")
	}

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("event=listen status=fail err=%v", err)
	}
	defer ln.Close()
	log.Printf("event=start mode=server listen=%s", *listen)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("event=accept status=fail err=%v", err)
			continue
		}
		go handleConn(conn, *token, *maxPayload, *udpTimeout, *dstIP, newRateLimiter(*ppsLimit, *bpsLimit))
	}
}

func handleConn(conn net.Conn, token string, maxPayload int, udpTimeout time.Duration, dstIP string, limiter *rateLimiter) {
	defer conn.Close()
	log.Printf("event=connect remote=%s", conn.RemoteAddr())

	udpConn, err := net.ListenUDP("udp", nil)
	if err != nil {
		log.Printf("event=udp_socket status=fail err=%v", err)
		return
	}
	defer udpConn.Close()

	var bytesIn, bytesOut uint64

	for {
		frame, err := framing.DecodeFrom(conn, token, maxPayload)
		if err != nil {
			log.Printf("event=disconnect remote=%s reason=%v bytes_in=%d bytes_out=%d", conn.RemoteAddr(), err, bytesIn, bytesOut)
			return
		}
		if frame.Header.Flags&framing.FlagKeepalive != 0 {
			continue
		}
		if frame.Header.Flags&framing.FlagData == 0 {
			continue
		}
		if !limiter.Allow(len(frame.Payload)) {
			log.Printf("event=rate_limit_drop remote=%s bytes=%d", conn.RemoteAddr(), len(frame.Payload))
			continue
		}
		if len(frame.Payload) > maxPayload {
			log.Printf("event=oversize_drop remote=%s bytes=%d max=%d", conn.RemoteAddr(), len(frame.Payload), maxPayload)
			continue
		}
		bytesIn += uint64(len(frame.Payload))

		dst := &net.UDPAddr{IP: net.ParseIP(dstIP), Port: int(frame.Header.DstPort)}
		if _, err := udpConn.WriteToUDP(frame.Payload, dst); err != nil {
			log.Printf("event=udp_forward status=fail err=%v dst=%s", err, dst)
			continue
		}

		buf := make([]byte, 65535)
		_ = udpConn.SetReadDeadline(time.Now().Add(udpTimeout))
		n, _, err := udpConn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			log.Printf("event=udp_read status=fail err=%v", err)
			continue
		}
		resp := framing.Frame{Header: framing.Header{Flags: framing.FlagData, SessionID: frame.Header.SessionID, DstPort: frame.Header.DstPort}, Payload: append([]byte(nil), buf[:n]...)}
		b, err := framing.Encode(resp, token, maxPayload)
		if err != nil {
			log.Printf("event=encode_drop err=%v", err)
			continue
		}
		if _, err := conn.Write(b); err != nil {
			log.Printf("event=write status=fail err=%v", err)
			return
		}
		bytesOut += uint64(n)
	}
}
