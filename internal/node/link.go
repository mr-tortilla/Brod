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

// ParseLink разбирает ссылку вида
// vless://uuid@host:port?type=tcp&security=reality&sni=...&pbk=...&sid=...&flow=...#name
func ParseLink(link string) (Node, error) {
	u, err := url.Parse(strings.TrimSpace(link))
	if err != nil {
		return Node{}, fmt.Errorf("некорректная ссылка: %w", err)
	}
	if u.Scheme != "vless" {
		return Node{}, fmt.Errorf("неподдерживаемый протокол %q", u.Scheme)
	}
	if u.User == nil || u.User.Username() == "" {
		return Node{}, errors.New("в ссылке нет UUID")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return Node{}, errors.New("в ссылке нет порта")
	}

	q := u.Query()
	n := Node{
		Name:        u.Fragment,
		Server:      u.Hostname(),
		Port:        port,
		UUID:        u.User.Username(),
		Flow:        q.Get("flow"),
		Network:     q.Get("type"),
		Security:    q.Get("security"),
		SNI:         q.Get("sni"),
		Fingerprint: q.Get("fp"),
		PublicKey:   q.Get("pbk"),
		ShortID:     q.Get("sid"),
		Path:        q.Get("path"),
		Host:        q.Get("host"),
		ServiceName: q.Get("serviceName"),
	}
	if alpn := q.Get("alpn"); alpn != "" {
		n.ALPN = strings.Split(alpn, ",")
	}
	n.finalize()
	return n, nil
}

// Link собирает vless:// ссылку — обратная операция к ParseLink.
// Такие ссылки понимают все VLESS-клиенты, в том числе мобильные через QR.
func Link(n Node) string {
	q := url.Values{}
	q.Set("encryption", "none")
	q.Set("type", n.Network)
	q.Set("security", n.Security)
	set := func(k, v string) {
		if v != "" {
			q.Set(k, v)
		}
	}
	set("flow", n.Flow)
	set("sni", n.SNI)
	set("fp", n.Fingerprint)
	set("pbk", n.PublicKey)
	set("sid", n.ShortID)
	set("path", n.Path)
	set("host", n.Host)
	set("serviceName", n.ServiceName)
	set("alpn", strings.Join(n.ALPN, ","))

	u := url.URL{
		Scheme:   "vless",
		User:     url.User(n.UUID),
		Host:     net.JoinHostPort(n.Server, strconv.Itoa(n.Port)),
		RawQuery: q.Encode(),
		Fragment: n.Name,
	}
	return u.String()
}

// ParseText разбирает тело подписки или вставленный текст:
// JSON, base64-список ссылок или просто ссылки построчно.
// Возвращает найденные VLESS-подключения и число пропущенных записей других протоколов.
func ParseText(text string) (nodes []Node, skipped int, err error) {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "{") || strings.HasPrefix(text, "[") {
		return ParseJSON([]byte(text))
	}
	if !strings.Contains(text, "://") {
		decoded, ok := decodeBase64(text)
		if !ok {
			return nil, 0, errors.New("не удалось распознать формат: ожидаются ссылки, base64 или JSON")
		}
		text = decoded
	}

	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "vless://"):
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
