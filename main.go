package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"

	"github.com/mr-tortilla/brod/internal/applog"
	"github.com/mr-tortilla/brod/internal/store"
	"github.com/mr-tortilla/brod/internal/subscription"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

// version подставляется при сборке: -ldflags "-X main.version=1.2.3" (см. Makefile).
// Статический анализ IDE этого не видит и считает version == "dev" всегда истинным — это не так:
// "dev" остаётся только у сборки без -X (make dev).
var version = "dev"

func main() {
	args := parseArgs(os.Args[1:])
	if args.waitPID > 0 {
		waitForExit(args.waitPID)
	}

	path, err := store.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	if err := applog.Init(filepath.Dir(path)); err != nil {
		log.Printf("лог в файл недоступен: %v", err)
	}
	log.Printf("Brod %s запущен, права администратора: %v", version, isElevated())
	s, err := store.Open(path)
	if err != nil {
		log.Fatalf("не удалось открыть %s: %v", path, err)
	}
	subscription.UserAgent = "Brod/" + version
	app := NewApp(s)
	app.startupConnect = args.connect

	err = wails.Run(&options.App{
		Title:     "Brod",
		Width:     1100,
		Height:    720,
		MinWidth:  800,
		MinHeight: 500,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 17, G: 20, B: 26, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		// Закрытие окна прячет его в трей, туннель продолжает работать. Выход — из меню трея.
		HideWindowOnClose: true,
		// Повторный запуск не создаёт второй экземпляр, а показывает окно уже запущенного:
		// это же спасает, если в системе нет трея (GNOME без расширения AppIndicator).
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               lockID(),
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.showWindow() },
		},
		Linux: &linux.Options{Icon: appIcon, ProgramName: "brod"},
		Bind: []any{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// lockID отличается у dev-сборки, чтобы её можно было запускать рядом с установленной версией.
func lockID() string {
	if version == "dev" {
		return "brod-7c1e9a52-dev"
	}
	return "brod-7c1e9a52"
}

type launchArgs struct {
	waitPID int            // дождаться выхода прежнего экземпляра (перезапуск с правами администратора)
	connect *StartupAction // подключиться сразу после запуска
}

// parseArgs разбирает аргументы, с которыми Brod перезапускает сам себя:
// --wait-pid <pid> --connect <профиль>:<подключение>:<режим>. Остальное игнорируется.
func parseArgs(argv []string) launchArgs {
	var a launchArgs
	for i := 0; i+1 < len(argv); i++ {
		switch argv[i] {
		case "--wait-pid":
			a.waitPID, _ = strconv.Atoi(argv[i+1])
		case "--connect":
			if parts := strings.SplitN(argv[i+1], ":", 3); len(parts) == 3 {
				a.connect = &StartupAction{ProfileID: parts[0], NodeID: parts[1], Mode: parts[2]}
			}
		}
	}
	return a
}
