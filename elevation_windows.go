package main

import (
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// isElevated: запущен ли процесс с правами администратора. Без них TUN на Windows не поднять.
func isElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// relaunchElevated запускает копию приложения с правами администратора (окно UAC).
func relaunchElevated(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = syscall.EscapeArg(a)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	params, _ := windows.UTF16PtrFromString(strings.Join(quoted, " "))
	return windows.ShellExecute(0, verb, file, params, nil, windows.SW_SHOWNORMAL)
}

// waitForExit ждёт завершения процесса pid (не дольше 15 секунд): перезапущенная копия
// должна дождаться выхода прежней, иначе упрётся в блокировку единственного экземпляра.
func waitForExit(pid int) {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return // процесса уже нет
	}
	defer windows.CloseHandle(h)
	_, _ = windows.WaitForSingleObject(h, 15000)
}
