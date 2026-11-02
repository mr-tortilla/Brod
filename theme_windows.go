package main

import "golang.org/x/sys/windows/registry"

// systemTheme на Windows: параметр «Режим приложений» в настройках персонализации.
func systemTheme() string {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	light, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return ""
	}
	if light == 0 {
		return "dark"
	}
	return "light"
}
