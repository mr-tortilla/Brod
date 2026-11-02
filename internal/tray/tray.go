// Package tray — значок Brod в системном трее: цвет показывает состояние подключения,
// меню позволяет открыть окно, отключиться и выйти.
package tray

import (
	"runtime"
	"sync"

	"fyne.io/systray"
)

type State int

const (
	Off  State = iota // приложение открыто, туннель не подключён (серый)
	On                // подключено (зелёный)
	Lost              // подключено, но связь через туннель пропала (красный)
)

type Callbacks struct {
	Open       func()
	Disconnect func()
	Quit       func()
}

type Tray struct {
	cb Callbacks

	mu      sync.Mutex
	ready   bool
	state   State
	tooltip string
	status  *systray.MenuItem
	disconn *systray.MenuItem
}

// Start запускает трей в отдельном потоке ОС: на Windows значку нужен свой цикл
// сообщений в том потоке, где создано его окно; на Linux трей работает через DBus.
func Start(cb Callbacks) *Tray {
	t := &Tray{cb: cb, tooltip: "Brod — отключено"}
	go func() {
		runtime.LockOSThread()
		systray.Run(t.onReady, nil)
	}()
	return t
}

func (t *Tray) Stop() {
	systray.Quit()
}

func (t *Tray) onReady() {
	systray.SetTitle("Brod")
	systray.SetOnTapped(t.cb.Open)

	open := systray.AddMenuItem("Открыть Brod", "")
	status := systray.AddMenuItem("", "")
	status.Disable()
	disconn := systray.AddMenuItem("Отключить", "")
	systray.AddSeparator()
	quit := systray.AddMenuItem("Выход", "Отключиться и закрыть приложение")

	t.mu.Lock()
	t.status, t.disconn, t.ready = status, disconn, true
	t.applyLocked()
	t.mu.Unlock()

	go func() {
		for {
			select {
			case <-open.ClickedCh:
				t.cb.Open()
			case <-disconn.ClickedCh:
				t.cb.Disconnect()
			case <-quit.ClickedCh:
				t.cb.Quit()
				return
			}
		}
	}()
}

// Set меняет значок и подписи. Вызывается на каждое обновление статуса (раз в секунду),
// поэтому значок перерисовывается только при смене состояния.
func (t *Tray) Set(state State, tooltip string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	changed := state != t.state || tooltip != t.tooltip
	t.state, t.tooltip = state, tooltip
	if t.ready && changed {
		t.applyLocked()
	}
}

func (t *Tray) applyLocked() {
	systray.SetIcon(icons[t.state])
	systray.SetTooltip(t.tooltip)
	t.status.SetTitle(t.tooltip)
	if t.state == Off {
		t.disconn.Disable()
	} else {
		t.disconn.Enable()
	}
}
