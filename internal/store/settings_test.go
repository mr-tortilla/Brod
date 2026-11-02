package store

import (
	"os"
	"path/filepath"
	"testing"
)

// Старый файл настроек (до появления новых полей) читается с дефолтами для недостающих полей,
// в том числе для bool, у которых дефолт — true.
func TestSettingsMissingFieldsGetDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"),
		[]byte(`{"bypass":["ru"],"directDns":"8.8.8.8","remoteDns":"1.1.1.1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Open(filepath.Join(dir, "profiles.json"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := s.Settings()
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultSettings()
	if st.DirectDNS != "8.8.8.8" || len(st.Bypass) != 1 {
		t.Fatalf("сохранённые значения потерялись: %+v", st)
	}
	if !st.BypassLAN || !st.SystemProxyInTUN || !st.RefreshOnConnect || st.MixedPort != def.MixedPort ||
		st.TunStack != def.TunStack || st.Theme != "auto" {
		t.Fatalf("дефолты не подставились: %+v", st)
	}
}

func TestSettingsValidation(t *testing.T) {
	for name, mutate := range map[string]func(*Settings){
		"порт": func(s *Settings) { s.MixedPort = 70000 },
		"MTU":  func(s *Settings) { s.TunMTU = 100 },
		"DNS":  func(s *Settings) { s.RemoteDNS = "dns.google" },
		"URL":  func(s *Settings) { s.TestURL = "ftp://x" },
		"стек": func(s *Settings) { s.TunStack = "lwip" },
		"тема": func(s *Settings) { s.Theme = "pink" },
		"ipv6": func(s *Settings) { s.IPv6 = "yes" },
	} {
		st := DefaultSettings()
		mutate(&st)
		if err := st.Normalize(); err == nil {
			t.Errorf("%s: ожидалась ошибка", name)
		}
	}
	st := DefaultSettings()
	st.MixedPort, st.RemoteDNS = 0, "  "
	if err := st.Normalize(); err != nil || st.RemoteDNS != DefaultSettings().RemoteDNS {
		t.Fatalf("порт 0 и пустой DNS допустимы: %v %+v", err, st)
	}
}
