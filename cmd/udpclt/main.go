package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"udp-custom-lite/internal/config"
	"udp-custom-lite/internal/framing"
	"udp-custom-lite/internal/session"
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
	cfgRaw := flag.String("config", "", "config: <serverHost>:<udpPortSpec>@<token>:<localPort>")
	dstRaw := flag.String("dst", "", "destination UDP <dst_ip:dst_port>")
	serverPort := flag.Int("server-port", 9000, "TCP server port")
	maxPayload := flag.Int("max-payload", framing.DefaultMaxPayload, "max payload bytes")
	keepalive := flag.Duration("keepalive", 15*time.Second, "keepalive interval")
	reconnect := flag.Duration("reconnect-delay", 2*time.Second, "reconnect delay")
	ppsLimit := flag.Int("rate-pps", 0, "egress packets/sec limit (0 disable)")
	bpsLimit := flag.Int("rate-bps", 0, "egress bytes/sec limit (0 disable)")
	flag.Parse()

	if *cfgRaw == "" || *dstRaw == "" {
		flag.Usage()
		os.Exit(2)
	}
	cfg, err := config.ParseClientConfig(*cfgRaw)
	if err != nil {
		log.Fatalf("event=config_parse status=fail err=%v", err)
	}
	dstAddr, err := net.ResolveUDPAddr("udp", *dstRaw)
	if err != nil {
		log.Fatalf("event=dst_parse status=fail err=%v", err)
	}
	if err := cfg.ValidateDstPort(uint16(dstAddr.Port)); err != nil {
		log.Fatalf("event=dst_validate status=fail err=%v", err)
	}

	localConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: int(cfg.LocalPort)})
	if err != nil {
		log.Fatalf("event=udp_listen status=fail err=%v", err)
	}
	defer localConn.Close()

	log.Printf("event=start mode=client udp_listen=%s server=%s:%d dst=%s any_udp_port=%t", localConn.LocalAddr(), cfg.ServerHost, *serverPort, dstAddr, cfg.AnyUDPPort)

	tbl := session.NewTable()
	outbound := make(chan framing.Frame, 1024)
	inbound := make(chan framing.Frame, 1024)
	limiter := newRateLimiter(*ppsLimit, *bpsLimit)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go readLocalUDP(ctx, localConn, tbl, outbound, uint16(dstAddr.Port), *maxPayload)
	go writeBackLocalUDP(ctx, localConn, tbl, inbound)

	runTunnel(ctx, fmt.Sprintf("%s:%d", cfg.ServerHost, *serverPort), cfg.Token, outbound, inbound, *keepalive, *reconnect, *maxPayload, limiter)
}

func readLocalUDP(ctx context.Context, conn *net.UDPConn, tbl *session.Table, outbound chan<- framing.Frame, dstPort uint16, maxPayload int) {
	buf := make([]byte, 65535)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
					continue
				}
			}
			log.Printf("event=udp_read status=fail err=%v", err)
			continue
		}
		if n > maxPayload {
			log.Printf("event=oversize_drop bytes=%d max=%d src=%s", n, maxPayload, addr)
			continue
		}
		sid, ok := tbl.SessionID(addr)
		if !ok {
			sid = randomSessionID()
			tbl.Set(sid, addr)
		}
		payload := make([]byte, n)
		copy(payload, buf[:n])
		frame := framing.Frame{Header: framing.Header{Flags: framing.FlagData, SessionID: sid, DstPort: dstPort}, Payload: payload}
		select {
		case outbound <- frame:
		default:
			log.Printf("event=queue_drop queue=outbound src=%s", addr)
		}
	}
}

func writeBackLocalUDP(ctx context.Context, conn *net.UDPConn, tbl *session.Table, inbound <-chan framing.Frame) {
	for {
		select {
		case <-ctx.Done():
			return
		case frame := <-inbound:
			if frame.Header.Flags&framing.FlagData == 0 {
				continue
			}
			addr, ok := tbl.Addr(frame.Header.SessionID)
			if !ok {
				continue
			}
			if _, err := conn.WriteToUDP(frame.Payload, addr); err != nil {
				log.Printf("event=udp_write status=fail err=%v dst=%s", err, addr)
			}
		}
	}
}

func runTunnel(ctx context.Context, serverAddr, token string, outbound <-chan framing.Frame, inbound chan<- framing.Frame, keepalive, reconnect time.Duration, maxPayload int, limiter *rateLimiter) {
	var bytesIn, bytesOut atomic.Uint64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		conn, err := net.Dial("tcp", serverAddr)
		if err != nil {
			log.Printf("event=connect status=fail server=%s err=%v", serverAddr, err)
			time.Sleep(reconnect)
			continue
		}
		log.Printf("event=connect status=ok server=%s", serverAddr)

		errCh := make(chan error, 1)
		go func() {
			for {
				frame, err := framing.DecodeFrom(conn, token, maxPayload)
				if err != nil {
					errCh <- err
					return
				}
				bytesIn.Add(uint64(len(frame.Payload)))
				select {
				case inbound <- frame:
				default:
					log.Printf("event=queue_drop queue=inbound")
				}
			}
		}()

		ticker := time.NewTicker(keepalive)
		idle := time.Now()
		running := true
		for running {
			select {
			case err := <-errCh:
				log.Printf("event=disconnect reason=%v bytes_in=%d bytes_out=%d", err, bytesIn.Load(), bytesOut.Load())
				running = false
			case frame := <-outbound:
				if !limiter.Allow(len(frame.Payload)) {
					log.Printf("event=rate_limit_drop bytes=%d", len(frame.Payload))
					continue
				}
				b, err := framing.Encode(frame, token, maxPayload)
				if err != nil {
					log.Printf("event=encode_drop err=%v", err)
					continue
				}
				if _, err := conn.Write(b); err != nil {
					log.Printf("event=write status=fail err=%v", err)
					running = false
					break
				}
				bytesOut.Add(uint64(len(frame.Payload)))
				idle = time.Now()
			case <-ticker.C:
				if time.Since(idle) < keepalive {
					continue
				}
				ka := framing.Frame{Header: framing.Header{Flags: framing.FlagKeepalive, SessionID: 0, DstPort: 1}}
				b, _ := framing.Encode(ka, token, maxPayload)
				if _, err := conn.Write(b); err != nil {
					log.Printf("event=keepalive status=fail err=%v", err)
					running = false
				}
			case <-ctx.Done():
				ticker.Stop()
				conn.Close()
				return
			}
		}
		ticker.Stop()
		conn.Close()
		time.Sleep(reconnect)
	}
}

func randomSessionID() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return uint64(time.Now().UnixNano())
	}
	return binary.BigEndian.Uint64(b[:])
}
