package main

import (
	"os/exec"
	"strings"

	"github.com/godbus/dbus/v5"
)

// systemTheme на Linux: сначала XDG Desktop Portal (GNOME, KDE и другие современные окружения),
// затем название GTK-темы.
func systemTheme() string {
	if t := portalTheme(); t != "" {
		return t
	}
	out, err := exec.Command("gsettings", "get", "org.gnome.desktop.interface", "gtk-theme").Output()
	if err != nil {
		return ""
	}
	if strings.Contains(strings.ToLower(string(out)), "dark") {
		return "dark"
	}
	return "light"
}

// portalTheme читает org.freedesktop.appearance color-scheme: 1 — тёмная, 2 — светлая, 0 — без предпочтения.
func portalTheme() string {
	// SessionBus — общее соединение на весь процесс (через него же работает трей),
	// поэтому его не закрываем: Close отключил бы и значок в трее.
	conn, err := dbus.SessionBus()
	if err != nil {
		return ""
	}
	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	var v dbus.Variant
	if err := obj.Call("org.freedesktop.portal.Settings.ReadOne", 0, "org.freedesktop.appearance", "color-scheme").Store(&v); err != nil {
		// старые версии портала: только Read, значение завёрнуто в ещё один variant
		if err := obj.Call("org.freedesktop.portal.Settings.Read", 0, "org.freedesktop.appearance", "color-scheme").Store(&v); err != nil {
			return ""
		}
	}
	for {
		inner, ok := v.Value().(dbus.Variant)
		if !ok {
			break
		}
		v = inner
	}
	switch v.Value() {
	case uint32(1):
		return "dark"
	case uint32(2):
		return "light"
	}
	return ""
}
