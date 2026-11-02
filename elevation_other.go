//go:build !windows

package main

import "errors"

// На Linux права для TUN выдаются бинарнику (setcap, это делает пакет), перезапуск не нужен.
func isElevated() bool { return true }

func relaunchElevated([]string) error {
	return errors.New("перезапуск с правами администратора поддерживается только на Windows")
}

func waitForExit(int) {}
