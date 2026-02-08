package config

import "testing"

func TestParseClientConfigAnyPort(t *testing.T) {
	cfg, err := ParseClientConfig("min.xhmt.my.id:1-65535@Trial25171:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.AnyUDPPort {
		t.Fatalf("expected AnyUDPPort=true")
	}
	if cfg.LocalPort != 1 || cfg.Token != "Trial25171" || cfg.ServerHost != "min.xhmt.my.id" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
	if err := cfg.ValidateDstPort(65535); err != nil {
		t.Fatalf("expected dst port valid: %v", err)
	}
}

func TestParseClientConfigRangePort(t *testing.T) {
	cfg, err := ParseClientConfig("min.xhmt.my.id:54-65535@Trial25171:1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AnyUDPPort {
		t.Fatalf("expected AnyUDPPort=false for partial range")
	}
	if cfg.PortMin != 54 || cfg.PortMax != 65535 {
		t.Fatalf("unexpected range: %d-%d", cfg.PortMin, cfg.PortMax)
	}
	if err := cfg.ValidateDstPort(53); err == nil {
		t.Fatalf("expected 53 to be rejected")
	}
	if err := cfg.ValidateDstPort(54); err != nil {
		t.Fatalf("expected 54 to be accepted: %v", err)
	}
}

func TestParseClientConfigSinglePort(t *testing.T) {
	cfg, err := ParseClientConfig("example.com:53@tok:9001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AnyUDPPort {
		t.Fatalf("expected AnyUDPPort=false")
	}
	if cfg.AllowedPort != 53 {
		t.Fatalf("unexpected allowed port: %d", cfg.AllowedPort)
	}
	if err := cfg.ValidateDstPort(53); err != nil {
		t.Fatalf("expected port 53 valid: %v", err)
	}
	if err := cfg.ValidateDstPort(54); err == nil {
		t.Fatalf("expected invalid port")
	}
}

func TestParseClientConfigTokenCanContainColon(t *testing.T) {
	cfg, err := ParseClientConfig("10.0.0.1:53@user:pass:5300")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "user:pass" {
		t.Fatalf("expected token user:pass got %q", cfg.Token)
	}
	if cfg.LocalPort != 5300 {
		t.Fatalf("expected localPort 5300 got %d", cfg.LocalPort)
	}
}

func TestParseClientConfigShorthandUserPass(t *testing.T) {
	cfg, err := ParseClientConfig("turu.kacer.store:1-65535@kacer:vpn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Token != "kacer:vpn" {
		t.Fatalf("expected token kacer:vpn got %q", cfg.Token)
	}
	if cfg.LocalPort != DefaultLocalPort {
		t.Fatalf("expected default local port %d got %d", DefaultLocalPort, cfg.LocalPort)
	}
}

func TestParseClientConfigInvalid(t *testing.T) {
	cases := []string{
		"",
		"badformat",
		"host:53@tok:0",
		":53@tok:1",
		"host:100-1@tok:1",
	}
	for _, c := range cases {
		if _, err := ParseClientConfig(c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}
