package core

import (
	"fmt"

	"github.com/mr-tortilla/brod/internal/node"
)

type Mode string

const (
	ModeProxy Mode = "proxy" // системный прокси, права не нужны
	ModeTUN   Mode = "tun"   // весь трафик через виртуальный интерфейс, нужны права
)

const (
	tagProxy  = "proxy"
	tagDirect = "direct"

	// Адреса TUN-интерфейса. Выбраны редкие, чтобы не пересекаться
	// с дефолтами других клиентов (172.19.0.1/fdfe:dcba:9876::1) и с сетями Docker.
	tunIPv4 = "172.30.199.1/30"
	tunIPv6 = "fd7e:2e1f:5c1a::1/126"
)

// DNS — серверы, которыми пользуется ядро.
type DNS struct {
	Direct string // для исключений и адреса самого сервера, ходит напрямую (UDP)
	Remote string // для всего остального, ходит через туннель (DoH)
}

// buildConfig собирает конфиг sing-box. Mixed-инбаунд поднимается всегда:
// через него приложение меряет задержку и узнаёт внешний IP; в режиме прокси он же
// прописывается системным прокси.
func buildConfig(n node.Node, opts Options, mixedPort int, cachePath string) (map[string]any, error) {
	outbound, err := vlessOutbound(n)
	if err != nil {
		return nil, err
	}
	bypass, dns := opts.Bypass, opts.DNS

	listen := "127.0.0.1"
	if opts.AllowLAN {
		listen = "0.0.0.0"
	}
	inbounds := []any{
		map[string]any{
			"type":        "mixed",
			"tag":         "mixed-in",
			"listen":      listen,
			"listen_port": mixedPort,
			// В режиме TUN системный прокси по умолчанию тоже ставится: иначе браузеры уйдут
			// в прокси, оставленный другим VPN-клиентом, и наш TUN увидит лишь соединение с его
			// сервером — двойной туннель, и исключения по доменам не сработают. Через прокси
			// браузер к тому же сразу передаёт домен, и правила по доменам срабатывают точнее.
			"set_system_proxy": opts.Mode == ModeProxy || opts.SystemProxyInTUN,
		},
	}
	if opts.Mode == ModeTUN {
		inbounds = append(inbounds, map[string]any{
			"type":           "tun",
			"tag":            "tun-in",
			"interface_name": "brod",
			"address":        []string{tunIPv4, tunIPv6},
			"mtu":            opts.TunMTU,
			"auto_route":     true,
			"strict_route":   opts.StrictRoute,
			"stack":          opts.TunStack,
		})
	}

	rules := []any{
		map[string]any{"action": "sniff"},
		map[string]any{"protocol": "dns", "action": "hijack-dns"},
	}
	if opts.BypassLAN {
		rules = append(rules, map[string]any{"ip_is_private": true, "outbound": tagDirect})
	}
	var dnsRules []any
	if len(bypass.Domains) > 0 {
		rules = append(rules, map[string]any{"domain_suffix": bypass.Domains, "outbound": tagDirect})
		// исключённые домены и резолвим напрямую, иначе сайт может получить «чужой» IP по геолокации
		dnsRules = append(dnsRules, map[string]any{"domain_suffix": bypass.Domains, "server": "local"})
	}
	if len(bypass.CIDRs) > 0 {
		rules = append(rules, map[string]any{"ip_cidr": bypass.CIDRs, "outbound": tagDirect})
	}

	return map[string]any{
		"log": map[string]any{"level": "warn"},
		"dns": map[string]any{
			"servers": []any{
				map[string]any{"type": "https", "tag": "remote", "server": dns.Remote, "detour": tagProxy},
				map[string]any{"type": "udp", "tag": "local", "server": dns.Direct},
			},
			"rules":    dnsRules,
			"final":    "remote",
			"strategy": opts.DNSStrategy,
		},
		"inbounds": inbounds,
		"outbounds": []any{
			outbound,
			map[string]any{"type": "direct", "tag": tagDirect},
		},
		"route": map[string]any{
			"rules":                   rules,
			"final":                   tagProxy,
			"auto_detect_interface":   true,
			"default_domain_resolver": "local",
		},
		// sing-box включает кэш принудительно, поэтому явно задаём ему место
		"experimental": map[string]any{
			"cache_file": map[string]any{"enabled": true, "path": cachePath},
		},
	}, nil
}

// Outbound — подключение в формате outbound sing-box с тегом-именем, для экспорта.
func Outbound(n node.Node) (map[string]any, error) {
	o, err := vlessOutbound(n)
	if err != nil {
		return nil, err
	}
	o["tag"] = n.Name
	return o, nil
}

func vlessOutbound(n node.Node) (map[string]any, error) {
	o := map[string]any{
		"type":            "vless",
		"tag":             tagProxy,
		"server":          n.Server,
		"server_port":     n.Port,
		"uuid":            n.UUID,
		"packet_encoding": "xudp",
	}
	if n.Flow != "" {
		o["flow"] = n.Flow
	}

	switch n.Security {
	case "none", "":
	case "tls", "reality":
		tls := map[string]any{"enabled": true}
		if n.SNI != "" {
			tls["server_name"] = n.SNI
		}
		if len(n.ALPN) > 0 {
			tls["alpn"] = n.ALPN
		}
		fp := n.Fingerprint
		if fp == "" && n.Security == "reality" {
			fp = "chrome" // reality без uTLS не работает
		}
		if fp != "" {
			tls["utls"] = map[string]any{"enabled": true, "fingerprint": fp}
		}
		if n.Security == "reality" {
			tls["reality"] = map[string]any{"enabled": true, "public_key": n.PublicKey, "short_id": n.ShortID}
		}
		o["tls"] = tls
	default:
		return nil, fmt.Errorf("шифрование %q не поддерживается", n.Security)
	}

	switch n.Network {
	case "tcp", "":
	case "ws", "httpupgrade":
		t := map[string]any{"type": n.Network, "path": n.Path}
		if n.Host != "" {
			if n.Network == "ws" {
				t["headers"] = map[string]any{"Host": n.Host}
			} else {
				t["host"] = n.Host
			}
		}
		o["transport"] = t
	case "grpc":
		o["transport"] = map[string]any{"type": "grpc", "service_name": n.ServiceName}
	case "http", "h2":
		t := map[string]any{"type": "http", "path": n.Path}
		if n.Host != "" {
			t["host"] = []string{n.Host}
		}
		o["transport"] = t
	default:
		return nil, fmt.Errorf("транспорт %q не поддерживается ядром sing-box", n.Network)
	}
	return o, nil
}
