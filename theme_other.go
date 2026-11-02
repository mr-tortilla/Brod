//go:build !linux && !windows

package main

// systemTheme на остальных системах не определяется: фронтенд берёт prefers-color-scheme.
func systemTheme() string {
	return ""
}
