package config

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const AnyUDPPortLiteral = "1-65535"

type ClientConfig struct {
	ServerHost  string
	AnyUDPPort  bool
	AllowedPort uint16
	Token       string
	LocalPort   uint16
}

func ParseClientConfig(raw string) (ClientConfig, error) {
	var cfg ClientConfig
	if strings.TrimSpace(raw) == "" {
		return cfg, fmt.Errorf("empty config")
	}

	at := strings.LastIndex(raw, "@")
	if at < 0 {
		return cfg, fmt.Errorf("invalid format: missing @")
	}
	left := raw[:at]
	right := raw[at+1:]

	host, portSpec, err := splitHostPortSpec(left)
	if err != nil {
		return cfg, err
	}
	cfg.ServerHost = host

	token, localPort, err := splitTokenLocalPort(right)
	if err != nil {
		return cfg, err
	}
	cfg.Token = token
	cfg.LocalPort = localPort

	if portSpec == AnyUDPPortLiteral {
		cfg.AnyUDPPort = true
		return cfg, nil
	}

	p, err := parsePort(portSpec)
	if err != nil {
		return cfg, fmt.Errorf("invalid udpPortSpec: %w", err)
	}
	cfg.AllowedPort = p
	return cfg, nil
}

func (c ClientConfig) ValidateDstPort(port uint16) error {
	if port == 0 {
		return fmt.Errorf("destination port must be 1..65535")
	}
	if c.AnyUDPPort {
		return nil
	}
	if port != c.AllowedPort {
		return fmt.Errorf("destination port %d not allowed; expected %d", port, c.AllowedPort)
	}
	return nil
}

func splitHostPortSpec(in string) (string, string, error) {
	idx := strings.LastIndex(in, ":")
	if idx <= 0 || idx == len(in)-1 {
		return "", "", fmt.Errorf("invalid <serverHost>:<udpPortSpec>")
	}
	host := in[:idx]
	portSpec := in[idx+1:]
	if strings.TrimSpace(host) == "" {
		return "", "", fmt.Errorf("empty serverHost")
	}
	if net.ParseIP(host) == nil {
		if strings.Contains(host, " ") {
			return "", "", fmt.Errorf("invalid serverHost")
		}
	}
	return host, portSpec, nil
}

func splitTokenLocalPort(in string) (string, uint16, error) {
	idx := strings.LastIndex(in, ":")
	if idx <= 0 || idx == len(in)-1 {
		return "", 0, fmt.Errorf("invalid <token>:<localPort>")
	}
	token := in[:idx]
	if token == "" {
		return "", 0, fmt.Errorf("empty token")
	}
	p, err := parsePort(in[idx+1:])
	if err != nil {
		return "", 0, fmt.Errorf("invalid localPort: %w", err)
	}
	return token, p, nil
}

func parsePort(s string) (uint16, error) {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("must be numeric")
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("must be 1..65535")
	}
	return uint16(n), nil
}
