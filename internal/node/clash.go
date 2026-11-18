package node

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Подписки и конфиги Clash / Mihomo — YAML со списком proxies.
var clashProxies = regexp.MustCompile(`(?m)^proxies:`)

func isClashYAML(text string) bool {
	return clashProxies.MatchString(text)
}

type clashProxy struct {
	Name              string   `yaml:"name"`
	Type              string   `yaml:"type"`
	Server            string   `yaml:"server"`
	Port              any      `yaml:"port"` // бывает и числом, и строкой
	UUID              string   `yaml:"uuid"`
	Password          string   `yaml:"password"`
	Flow              string   `yaml:"flow"`
	Network           string   `yaml:"network"`
	TLS               bool     `yaml:"tls"`
	ServerName        string   `yaml:"servername"`
	SNI               string   `yaml:"sni"`
	ClientFingerprint string   `yaml:"client-fingerprint"`
	ALPN              []string `yaml:"alpn"`
	RealityOpts       struct {
		PublicKey string `yaml:"public-key"`
		ShortID   string `yaml:"short-id"`
	} `yaml:"reality-opts"`
	WSOpts struct {
		Path             string            `yaml:"path"`
		Headers          map[string]string `yaml:"headers"`
		V2rayHTTPUpgrade bool              `yaml:"v2ray-http-upgrade"`
	} `yaml:"ws-opts"`
	GRPCOpts struct {
		ServiceName string `yaml:"grpc-service-name"`
	} `yaml:"grpc-opts"`
	H2Opts struct {
		Host []string `yaml:"host"`
		Path string   `yaml:"path"`
	} `yaml:"h2-opts"`
	HTTPOpts struct {
		Path    []string            `yaml:"path"`
		Headers map[string][]string `yaml:"headers"`
	} `yaml:"http-opts"`
	UpMbps            any    `yaml:"up"`
	DownMbps          any    `yaml:"down"`
	Obfs              string `yaml:"obfs"`
	ObfsPassword      string `yaml:"obfs-password"`
	CongestionControl string `yaml:"congestion-controller"`
	UDPRelayMode      string `yaml:"udp-relay-mode"`
	SkipCertVerify    bool   `yaml:"skip-cert-verify"`
}

// ParseClash разбирает YAML Clash / Mihomo и поддерживает VLESS, Hysteria2 и TUIC.
func ParseClash(text string) (nodes []Node, skipped int, err error) {
	var doc struct {
		Proxies []clashProxy `yaml:"proxies"`
	}
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, 0, fmt.Errorf("некорректный YAML: %w", err)
	}
	for _, p := range doc.Proxies {
		if p.Type != "vless" && p.Type != "hysteria2" && p.Type != "tuic" {
			skipped++
			continue
		}
		n, ok := fromClash(p)
		if !ok {
			skipped++
			continue
		}
		nodes = append(nodes, n)
	}
	return uniqueIDs(nodes), skipped, nil
}

func parseMbps(value any) int {
	if value == nil {
		return 0
	}
	text := strings.TrimSpace(strings.ToLower(fmt.Sprint(value)))
	if text == "" {
		return 0
	}
	fields := strings.Fields(text)
	number, err := strconv.ParseFloat(strings.TrimRight(fields[0], "abcdefghijklmnopqrstuvwxyz"), 64)
	if err != nil || number <= 0 {
		return 0
	}
	unit := "mbps"
	if len(fields) > 1 {
		unit = strings.TrimSpace(fields[1])
	}
	switch unit {
	case "gbps":
		number *= 1000
	case "kbps":
		number /= 1000
	case "bps":
		number /= 1_000_000
	}
	return int(number + 0.5)
}

func fromClash(p clashProxy) (Node, bool) {
	port, err := strconv.Atoi(fmt.Sprint(p.Port))
	if err != nil || p.Server == "" {
		return Node{}, false
	}
	if (p.Type == "vless" || p.Type == "tuic") && p.UUID == "" || p.Type == "hysteria2" && p.Password == "" {
		return Node{}, false
	}
	n := Node{
		Protocol:    p.Type,
		Name:        p.Name,
		Server:      p.Server,
		Port:        port,
		UUID:        p.UUID,
		Password:    p.Password,
		Flow:        p.Flow,
		Network:     p.Network,
		Fingerprint: p.ClientFingerprint,
		ALPN:        p.ALPN,
		SNI:         p.ServerName,
	}
	n.UpMbps, n.DownMbps = parseMbps(p.UpMbps), parseMbps(p.DownMbps)
	n.ObfsType, n.ObfsPassword = p.Obfs, p.ObfsPassword
	n.CongestionControl, n.UDPRelayMode, n.Insecure = p.CongestionControl, p.UDPRelayMode, p.SkipCertVerify
	if n.SNI == "" {
		n.SNI = p.SNI
	}
	if p.Type == "hysteria2" || p.Type == "tuic" {
		n.ALPN = p.ALPN
		n.finalize()
		return n, true
	}
	switch {
	case p.RealityOpts.PublicKey != "":
		n.Security, n.PublicKey, n.ShortID = "reality", p.RealityOpts.PublicKey, p.RealityOpts.ShortID
	case p.TLS:
		n.Security = "tls"
	}

	switch p.Network {
	case "ws":
		n.Path, n.Host = p.WSOpts.Path, p.WSOpts.Headers["Host"]
		if p.WSOpts.V2rayHTTPUpgrade {
			n.Network = "httpupgrade"
		}
	case "grpc":
		n.ServiceName = p.GRPCOpts.ServiceName
	case "h2":
		n.Network, n.Path = "http", p.H2Opts.Path
		if len(p.H2Opts.Host) > 0 {
			n.Host = p.H2Opts.Host[0]
		}
	case "http":
		// в Clash network: http — это TCP с HTTP-маскировкой, sing-box её не поддерживает
		return Node{}, false
	}
	n.finalize()
	return n, true
}
