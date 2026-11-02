package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdlog "log"
	"net"
	"path/filepath"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/trafficcontrol"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	sjson "github.com/sagernet/sing/common/json"
	"github.com/sagernet/sing/service"

	"github.com/mr-tortilla/brod/internal/node"
)

// Core — запущенный экземпляр sing-box.
type Core struct {
	box       *box.Box
	cancel    context.CancelFunc
	traffic   *trafficcontrol.Manager
	testURL   string
	MixedPort int
}

// Options — всё, что задаётся настройками; значения по умолчанию — в store.DefaultSettings.
type Options struct {
	Mode    Mode
	Bypass  Bypass
	DNS     DNS
	DataDir string // каталог для служебных файлов sing-box

	Port             int    // порт локального прокси; 0 — любой свободный
	AllowLAN         bool   // принимать подключения к прокси из локальной сети
	BypassLAN        bool   // частные сети (192.168.x.x и т.п.) напрямую, мимо туннеля
	DNSStrategy      string // ipv4_only, prefer_ipv4, prefer_ipv6
	SystemProxyInTUN bool   // ставить системный прокси и в режиме TUN
	TunStack         string // system, gvisor, mixed
	TunMTU           int
	StrictRoute      bool
	TestURL          string // адрес для замера задержки
}

func Start(n node.Node, opts Options) (*Core, error) {
	port := opts.Port
	if port == 0 {
		var err error
		if port, err = freePort(); err != nil {
			return nil, err
		}
	}
	cfg, err := buildConfig(n, opts, port, filepath.Join(opts.DataDir, "cache.db"))
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(include.Context(context.Background()))
	options, err := sjson.UnmarshalExtendedContext[option.Options](ctx, data)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("конфиг ядра: %w", err)
	}
	instance, err := box.New(box.Options{
		Context: ctx,
		Options: options,
		// Без PlatformLogWriter sing-box не создаёт счётчик трафика.
		PlatformLogWriter: logWriter{},
	})
	if err != nil {
		cancel()
		return nil, err
	}
	if err := instance.Start(); err != nil {
		// ядро могло успеть поставить системный прокси или поднять TUN — ошибку отката не теряем
		err = errors.Join(err, instance.Close())
		cancel()
		return nil, err
	}
	return &Core{
		box:       instance,
		cancel:    cancel,
		traffic:   service.PtrFromContext[trafficcontrol.Manager](ctx),
		testURL:   opts.TestURL,
		MixedPort: port,
	}, nil
}

// Traffic возвращает суммарные байты с момента запуска.
func (c *Core) Traffic() (up, down int64) {
	if c.traffic == nil {
		return 0, 0
	}
	return c.traffic.Total()
}

func (c *Core) Close() error {
	defer c.cancel()
	return c.box.Close()
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

type logWriter struct{}

func (logWriter) WriteMessage(level log.Level, message string) {
	if level > log.LevelWarn {
		return
	}
	stdlog.Printf("[sing-box %s] %s", log.FormatLevel(level), message)
}
