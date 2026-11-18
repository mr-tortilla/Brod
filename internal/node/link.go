package node

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParseLink разбирает ссылки VLESS, Hysteria2 и TUIC.
func ParseLink(link string) (Node, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return Node{}, fmt.Errorf("некорректная ссылка: %w", err)
	}
	protocol := strings.ToLower(u.Scheme)
	if protocol == "hy2" {
		protocol = "hysteria2"
	}
	if protocol != "vless" && protocol != "hysteria2" && protocol != "tuic" {
		return Node{}, fmt.Errorf("неподдерживаемый протокол %q", u.Scheme)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Node{}, errors.New("в ссылке нет порта")
	}
	q := u.Query()
	n := Node{Protocol: protocol, Name: u.Fragment, Server: u.Hostname(), Port: port}
	if protocol == "vless" {
		if u.User == nil || u.User.Username() == "" {
			return Node{}, errors.New("в ссылке нет UUID")
		}
		n.UUID = u.User.Username()
		n.Flow, n.Network, n.Security = q.Get("flow"), q.Get("type"), q.Get("security")
		n.SNI, n.Fingerprint, n.PublicKey, n.ShortID = q.Get("sni"), q.Get("fp"), q.Get("pbk"), q.Get("sid")
		n.Path, n.Host, n.ServiceName = q.Get("path"), q.Get("host"), q.Get("serviceName")
		if alpn := q.Get("alpn"); alpn != "" {
			n.ALPN = strings.Split(alpn, ",")
		}
	} else if protocol == "hysteria2" {
		if u.User == nil || u.User.Username() == "" {
			return Node{}, errors.New("в ссылке нет пароля")
		}
		n.Password = u.User.Username()
		if pass, ok := u.User.Password(); ok {
			n.Password += ":" + pass
		}
		n.SNI, n.Insecure = q.Get("sni"), queryBool(q, "insecure", "allowInsecure")
		n.ObfsType, n.ObfsPassword = q.Get("obfs"), q.Get("obfs-password")
		n.UpMbps, _ = strconv.Atoi(q.Get("upmbps"))
		n.DownMbps, _ = strconv.Atoi(q.Get("downmbps"))
		if alpn := q.Get("alpn"); alpn != "" {
			n.ALPN = strings.Split(alpn, ",")
		}
	} else {
		if u.User == nil || u.User.Username() == "" {
			return Node{}, errors.New("в ссылке нет UUID")
		}
		n.UUID = u.User.Username()
		n.Password, _ = u.User.Password()
		n.SNI, n.Insecure = q.Get("sni"), queryBool(q, "allow_insecure", "allowInsecure")
		n.CongestionControl, n.UDPRelayMode = q.Get("congestion_control"), q.Get("udp_relay_mode")
		if alpn := q.Get("alpn"); alpn != "" {
			n.ALPN = strings.Split(alpn, ",")
		}
	}
	n.finalize()
	return n, nil
}

func queryBool(q url.Values, keys ...string) bool {
	for _, key := range keys {
		v := strings.ToLower(q.Get(key))
		if v == "1" || v == "true" || v == "yes" {
			return true
		}
	}
	return false
}

// Link возвращает URI, совместимый с клиентами соответствующего протокола.
func Link(n Node) string {
	protocol := n.Protocol
	if protocol == "" {
		protocol = "vless"
	}
	q := url.Values{}
	var user *url.Userinfo
	switch protocol {
	case "hysteria2":
		user = url.User(n.Password)
		setQuery(q, "sni", n.SNI)
		setQuery(q, "obfs", n.ObfsType)
		setQuery(q, "obfs-password", n.ObfsPassword)
		if n.Insecure {
			q.Set("insecure", "1")
		}
		if n.UpMbps > 0 {
			q.Set("upmbps", strconv.Itoa(n.UpMbps))
		}
		if n.DownMbps > 0 {
			q.Set("downmbps", strconv.Itoa(n.DownMbps))
		}
		if len(n.ALPN) > 0 {
			q.Set("alpn", strings.Join(n.ALPN, ","))
		}
	case "tuic":
		user = url.UserPassword(n.UUID, n.Password)
		setQuery(q, "sni", n.SNI)
		setQuery(q, "congestion_control", n.CongestionControl)
		setQuery(q, "udp_relay_mode", n.UDPRelayMode)
		if n.Insecure {
			q.Set("allow_insecure", "1")
		}
		if len(n.ALPN) > 0 {
			q.Set("alpn", strings.Join(n.ALPN, ","))
		}
	default:
		q.Set("encryption", "none")
		q.Set("type", n.Network)
		q.Set("security", n.Security)
		user = url.User(n.UUID)
		setQuery(q, "flow", n.Flow)
		setQuery(q, "sni", n.SNI)
		setQuery(q, "fp", n.Fingerprint)
		setQuery(q, "pbk", n.PublicKey)
		setQuery(q, "sid", n.ShortID)
		setQuery(q, "path", n.Path)
		setQuery(q, "host", n.Host)
		setQuery(q, "serviceName", n.ServiceName)
		if len(n.ALPN) > 0 {
			q.Set("alpn", strings.Join(n.ALPN, ","))
		}
	}
	u := url.URL{Scheme: protocol, User: user, Host: net.JoinHostPort(n.Server, strconv.Itoa(n.Port)), RawQuery: q.Encode(), Fragment: n.Name}
	return u.String()
}
func setQuery(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

// ParseText разбирает тело подписки или вставленный текст:
// JSON (xray, sing-box), YAML (Clash, Mihomo), base64-список ссылок или просто ссылки построчно.
// Возвращает найденные подключения и число пропущенных записей неподдерживаемых протоколов.
func ParseText(text string) (nodes []Node, skipped int, err error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return ParseJSON([]byte(text))
	}
	if isClashYAML(text) {
		return ParseClash(text)
	}
	if !strings.Contains(text, "://") {
		decoded, ok := decodeBase64(text)
		if !ok {
			return nil, 0, errors.New("не удалось распознать формат: ожидаются ссылки, base64, JSON или YAML (Clash)")
		}
		// base64 может скрывать и YAML
		if isClashYAML(decoded) {
			return ParseClash(decoded)
		}
		text = decoded
	}

	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "vless://"), strings.HasPrefix(line, "hysteria2://"), strings.HasPrefix(line, "hy2://"), strings.HasPrefix(line, "tuic://"):
			n, err := ParseLink(line)
			if err != nil {
				skipped++
				continue
			}
			nodes = append(nodes, n)
		default:
			skipped++
		}
	}
	return uniqueIDs(nodes), skipped, nil
}

func decodeBase64(s string) (string, bool) {
	s = strings.Join(strings.Fields(s), "")
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil {
			return string(b), true
		}
	}
	return "", false
}
