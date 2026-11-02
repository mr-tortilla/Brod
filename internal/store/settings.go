package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Settings struct {
	// Подключение
	MixedPort        int    `json:"mixedPort"`        // порт локального прокси; 0 — любой свободный
	AllowLAN         bool   `json:"allowLan"`         // пускать к прокси устройства из локальной сети
	BypassLAN        bool   `json:"bypassLan"`        // локальная сеть напрямую, мимо туннеля
	RefreshOnConnect bool   `json:"refreshOnConnect"` // обновлять подписку перед подключением
	TestURL          string `json:"testUrl"`          // адрес для замера задержки

	// TUN
	SystemProxyInTUN bool   `json:"systemProxyInTun"` // ставить системный прокси и в режиме TUN
	TunStack         string `json:"tunStack"`         // system, gvisor, mixed
	TunMTU           int    `json:"tunMtu"`
	StrictRoute      bool   `json:"strictRoute"`

	// DNS
	DirectDNS string `json:"directDns"` // для исключений и адресов серверов, запросы идут напрямую
	RemoteDNS string `json:"remoteDns"` // для всего остального, через туннель по DoH
	IPv6      string `json:"ipv6"`      // off, prefer_ipv4, prefer_ipv6

	// Исключения: домены, зоны, IP и подсети, которые идут напрямую, мимо туннеля.
	Bypass []string `json:"bypass"`

	// Интерфейс
	Theme string `json:"theme"` // auto, light, dark
}

// DefaultSettings — значения по умолчанию. Они же подставляются для полей,
// которых нет в файле (например, после обновления приложения).
func DefaultSettings() Settings {
	return Settings{
		MixedPort:        2080,
		BypassLAN:        true,
		RefreshOnConnect: true,
		TestURL:          "http://cp.cloudflare.com/generate_204",
		SystemProxyInTUN: true,
		TunStack:         "mixed",
		TunMTU:           9000,
		StrictRoute:      true,
		DirectDNS:        "77.88.8.8",
		RemoteDNS:        "1.1.1.1",
		// IPv6 выключен: многие серверы не выпускают IPv6, и приложения, получив AAAA-запись,
		// сначала долго пытаются достучаться по IPv6 через туннель.
		IPv6:   "off",
		Bypass: []string{},
		Theme:  "auto",
	}
}

var (
	tunStacks = []string{"system", "gvisor", "mixed"}
	ipv6Modes = []string{"off", "prefer_ipv4", "prefer_ipv6"}
	themes    = []string{"auto", "light", "dark"}
)

// Normalize обрезает пробелы, подставляет значения по умолчанию в пустые поля
// и проверяет остальные.
func (st *Settings) Normalize() error {
	def := DefaultSettings()
	trim := func(p *string, fallback string) {
		if *p = strings.TrimSpace(*p); *p == "" {
			*p = fallback
		}
	}
	trim(&st.DirectDNS, def.DirectDNS)
	trim(&st.RemoteDNS, def.RemoteDNS)
	trim(&st.TestURL, def.TestURL)
	trim(&st.TunStack, def.TunStack)
	trim(&st.IPv6, def.IPv6)
	trim(&st.Theme, def.Theme)
	if st.TunMTU == 0 {
		st.TunMTU = def.TunMTU
	}
	if st.Bypass == nil {
		st.Bypass = []string{}
	}

	if st.MixedPort < 0 || st.MixedPort > 65535 {
		return errors.New("порт: от 1 до 65535, или 0 — выбирать свободный автоматически")
	}
	if st.TunMTU < 1280 || st.TunMTU > 65535 {
		return errors.New("MTU: от 1280 до 65535")
	}
	for _, d := range []struct{ name, value string }{{"DNS через туннель", st.RemoteDNS}, {"прямой DNS", st.DirectDNS}} {
		if _, err := netip.ParseAddr(d.value); err != nil {
			return fmt.Errorf("%s: нужен IP-адрес, например 1.1.1.1", d.name)
		}
	}
	if u, err := url.Parse(st.TestURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("адрес проверки задержки: нужна ссылка http:// или https://")
	}
	if !slices.Contains(tunStacks, st.TunStack) {
		return fmt.Errorf("стек TUN: одно из %s", strings.Join(tunStacks, ", "))
	}
	if !slices.Contains(ipv6Modes, st.IPv6) {
		return fmt.Errorf("IPv6: одно из %s", strings.Join(ipv6Modes, ", "))
	}
	if !slices.Contains(themes, st.Theme) {
		return fmt.Errorf("тема: одна из %s", strings.Join(themes, ", "))
	}
	return nil
}

func (s *Store) settingsPath() string {
	return filepath.Join(s.Dir(), "settings.json")
}

// Settings читает настройки поверх значений по умолчанию: отсутствующие в файле поля
// получают дефолты, а не нулевые значения.
func (s *Store) Settings() (Settings, error) {
	st := DefaultSettings()
	data, err := os.ReadFile(s.settingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return st, err
	}
	if err := st.Normalize(); err != nil {
		// повреждённое значение в файле не должно ломать запуск
		return DefaultSettings(), nil
	}
	return st, nil
}

func (s *Store) SaveSettings(st Settings) error {
	if err := st.Normalize(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(s.settingsPath(), data)
}
