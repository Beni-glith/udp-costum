package framing

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	f := Frame{Header: Header{Flags: FlagData, SessionID: 42, DstPort: 53}, Payload: []byte("hello")}
	b, err := Encode(f, "secret", DefaultMaxPayload)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := DecodeFrom(bytes.NewReader(b), "secret", DefaultMaxPayload)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Header.DstPort != 53 || out.Header.SessionID != 42 || string(out.Payload) != "hello" {
		t.Fatalf("unexpected out: %+v", out)
	}
}

func TestDecodeInvalidHMAC(t *testing.T) {
	f := Frame{Header: Header{Flags: FlagData, SessionID: 7, DstPort: 1234}, Payload: []byte("abc")}
	b, err := Encode(f, "secret", DefaultMaxPayload)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	b[len(b)-1] ^= 0xFF
	_, err = DecodeFrom(bytes.NewReader(b), "secret", DefaultMaxPayload)
	if err != ErrInvalidHMAC {
		t.Fatalf("expected ErrInvalidHMAC got %v", err)
	}
}

func TestDecodeInvalidLength(t *testing.T) {
	f := Frame{Header: Header{Flags: FlagData, SessionID: 1, DstPort: 9999}, Payload: make([]byte, 16)}
	b, err := Encode(f, "secret", DefaultMaxPayload)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	binary.BigEndian.PutUint16(b[16:18], 2000)
	_, err = DecodeFrom(bytes.NewReader(b), "secret", 100)
	if err == nil {
		t.Fatalf("expected invalid length error")
	}
}

func TestEncodeDstPortZero(t *testing.T) {
	_, err := Encode(Frame{Header: Header{Flags: FlagData, SessionID: 1, DstPort: 0}, Payload: []byte("x")}, "secret", DefaultMaxPayload)
	if err == nil {
		t.Fatalf("expected error for dst port zero")
	}
}
