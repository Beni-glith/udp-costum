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

func TestParseClientConfigInvalid(t *testing.T) {
	cases := []string{
		"",
		"badformat",
		"host:53@tok:0",
		":53@tok:1",
	}
	for _, c := range cases {
		if _, err := ParseClientConfig(c); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}
