package framing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	Magic        = "UDPC"
	Version byte = 1

	FlagData      byte = 0x01
	FlagKeepalive byte = 0x02

	HeaderLen         = 18
	HMACLen           = 32
	TrailerLen        = 1 + HMACLen
	DefaultMaxPayload = 1200
)

var (
	ErrInvalidMagic   = errors.New("invalid magic")
	ErrInvalidVersion = errors.New("invalid version")
	ErrInvalidHMAC    = errors.New("invalid hmac")
	ErrInvalidLength  = errors.New("invalid length")
)

type Header struct {
	Flags      byte
	SessionID  uint64
	DstPort    uint16
	PayloadLen uint16
}

type Frame struct {
	Header  Header
	Payload []byte
}

func Encode(frame Frame, token string, maxPayload int) ([]byte, error) {
	if maxPayload <= 0 {
		maxPayload = DefaultMaxPayload
	}
	if len(frame.Payload) > maxPayload {
		return nil, fmt.Errorf("payload exceeds limit: %d > %d", len(frame.Payload), maxPayload)
	}
	if len(frame.Payload) > 0xFFFF {
		return nil, ErrInvalidLength
	}
	if frame.Header.DstPort == 0 {
		return nil, fmt.Errorf("dst port must be 1..65535")
	}

	frame.Header.PayloadLen = uint16(len(frame.Payload))
	buf := make([]byte, HeaderLen+len(frame.Payload)+TrailerLen)

	copy(buf[0:4], []byte(Magic))
	buf[4] = Version
	buf[5] = frame.Header.Flags
	binary.BigEndian.PutUint64(buf[6:14], frame.Header.SessionID)
	binary.BigEndian.PutUint16(buf[14:16], frame.Header.DstPort)
	binary.BigEndian.PutUint16(buf[16:18], frame.Header.PayloadLen)
	copy(buf[18:18+len(frame.Payload)], frame.Payload)

	buf[18+len(frame.Payload)] = HMACLen
	sig := sign(buf[:18+len(frame.Payload)], []byte(token))
	copy(buf[19+len(frame.Payload):], sig)

	return buf, nil
}

func DecodeFrom(r io.Reader, token string, maxPayload int) (Frame, error) {
	var out Frame
	if maxPayload <= 0 {
		maxPayload = DefaultMaxPayload
	}

	headerBytes := make([]byte, HeaderLen)
	if _, err := io.ReadFull(r, headerBytes); err != nil {
		return out, err
	}
	if string(headerBytes[0:4]) != Magic {
		return out, ErrInvalidMagic
	}
	if headerBytes[4] != Version {
		return out, ErrInvalidVersion
	}

	out.Header.Flags = headerBytes[5]
	out.Header.SessionID = binary.BigEndian.Uint64(headerBytes[6:14])
	out.Header.DstPort = binary.BigEndian.Uint16(headerBytes[14:16])
	out.Header.PayloadLen = binary.BigEndian.Uint16(headerBytes[16:18])

	if int(out.Header.PayloadLen) > maxPayload {
		return out, fmt.Errorf("%w: payload %d > %d", ErrInvalidLength, out.Header.PayloadLen, maxPayload)
	}

	payload := make([]byte, out.Header.PayloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return out, err
	}
	trailer := make([]byte, TrailerLen)
	if _, err := io.ReadFull(r, trailer); err != nil {
		return out, err
	}
	if trailer[0] != HMACLen {
		return out, ErrInvalidLength
	}

	msg := append(headerBytes, payload...)
	expected := sign(msg, []byte(token))
	if !hmac.Equal(expected, trailer[1:]) {
		return out, ErrInvalidHMAC
	}

	out.Payload = payload
	return out, nil
}

func sign(data, key []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}
