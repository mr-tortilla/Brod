package core

import (
	"net/netip"
	"regexp"
	"strings"

	"golang.org/x/net/idna"
)

var domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

// Bypass — разобранный список исключений: что ходит напрямую, мимо туннеля.
type Bypass struct {
	Domains []string // суффиксы: "org" покрывает всю зону *.org, "example.com" — сайт и все поддомены
	CIDRs   []string
}

// ParseBypass понимает домены (в том числе на национальных алфавитах, они переводятся в punycode),
// зоны (org, .org, *.org), ссылки (https://example.com/path), IP и подсети.
// Нераспознанные строки возвращает в invalid.
func ParseBypass(lines []string) (b Bypass, invalid []string) {
	seen := map[string]bool{}
	for _, raw := range lines {
		s := strings.ToLower(strings.TrimSpace(raw))
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		if i := strings.Index(s, "://"); i >= 0 {
			s = s[i+3:]
		}
		if i := strings.IndexAny(s, "/?#"); i >= 0 && !isCIDR(s) {
			s = s[:i]
		}
		s = strings.TrimPrefix(strings.TrimPrefix(s, "*"), ".")

		switch {
		case isCIDR(s):
			p, _ := netip.ParsePrefix(s)
			s = p.Masked().String()
			if !seen[s] {
				b.CIDRs = append(b.CIDRs, s)
			}
		case isIP(s):
			a, _ := netip.ParseAddr(s)
			s = netip.PrefixFrom(a, a.BitLen()).String()
			if !seen[s] {
				b.CIDRs = append(b.CIDRs, s)
			}
		default:
			if h, _, ok := strings.Cut(s, ":"); ok { // example.com:8080
				s = h
			}
			ascii, err := idna.Lookup.ToASCII(s)
			if err != nil || !domainRe.MatchString(ascii) {
				invalid = append(invalid, raw)
				continue
			}
			s = ascii
			if !seen[s] {
				b.Domains = append(b.Domains, s)
			}
		}
		seen[s] = true
	}
	return b, invalid
}

func isCIDR(s string) bool {
	_, err := netip.ParsePrefix(s)
	return err == nil
}

func isIP(s string) bool {
	_, err := netip.ParseAddr(s)
	return err == nil
}
