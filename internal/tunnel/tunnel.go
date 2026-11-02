package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mr-tortilla/brod/internal/core"
	"github.com/mr-tortilla/brod/internal/node"
)

type State string

const (
	StateDisconnected State = "disconnected"
	StateConnecting   State = "connecting"
	StateConnected    State = "connected"
)

const (
	tick          = time.Second
	latencyTO     = 5 * time.Second
	ipInfoTO      = 10 * time.Second
	ipInfoRetry   = 10 * time.Second
	ipInfoRefresh = 5 * time.Minute
)

type Status struct {
	State     State     `json:"state"`
	Mode      core.Mode `json:"mode"`
	ProfileID string    `json:"profileId"`
	NodeID    string    `json:"nodeId"`
	NodeName  string    `json:"nodeName"`
	Server    string    `json:"server"`
	Since     int64     `json:"since"` // unix ms

	UpSpeed   int64 `json:"upSpeed"` // байт/с
	DownSpeed int64 `json:"downSpeed"`
	UpTotal   int64 `json:"upTotal"`
	DownTotal int64 `json:"downTotal"`
	LatencyMs int   `json:"latencyMs"` // 0 — ещё не измерено, -1 — ошибка

	IP          string `json:"ip"`
	CountryCode string `json:"countryCode"`
	City        string `json:"city"`
	Org         string `json:"org"`
}

// Manager держит не больше одного активного подключения и раз в секунду
// публикует его статус через onStatus.
type Manager struct {
	dataDir  string
	onStatus func(Status)

	opMu sync.Mutex // сериализует Connect/Disconnect

	mu     sync.Mutex
	core   *core.Core
	cancel context.CancelFunc
	status Status
}

func NewManager(dataDir string, onStatus func(Status)) *Manager {
	return &Manager{dataDir: dataDir, onStatus: onStatus, status: Status{State: StateDisconnected}}
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// Connect поднимает подключение; opts.DataDir проставляет сам менеджер.
func (m *Manager) Connect(profileID string, n node.Node, opts core.Options) error {
	mode := opts.Mode
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.disconnect()

	m.update(func(s *Status) {
		*s = Status{State: StateConnecting, Mode: mode, ProfileID: profileID, NodeID: n.ID, NodeName: n.Name, Server: n.Server}
	})

	opts.DataDir = m.dataDir
	c, err := core.Start(n, opts)
	if err != nil {
		m.update(func(s *Status) { *s = Status{State: StateDisconnected} })
		return friendlyError(err, opts)
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.core, m.cancel = c, cancel
	m.mu.Unlock()

	m.update(func(s *Status) {
		s.State = StateConnected
		s.Since = time.Now().UnixMilli()
	})
	go m.monitor(ctx, c)
	return nil
}

func (m *Manager) Disconnect() {
	m.opMu.Lock()
	defer m.opMu.Unlock()
	m.disconnect()
}

func (m *Manager) disconnect() {
	m.mu.Lock()
	c, cancel := m.core, m.cancel
	m.core, m.cancel = nil, nil
	m.mu.Unlock()
	if c == nil {
		return
	}
	cancel()
	// при закрытии ядро снимает системный прокси и маршруты TUN — если не вышло,
	// в системе мог остаться прокси на закрытый порт, это должно быть видно в логе
	if err := c.Close(); err != nil {
		log.Printf("ошибка при остановке ядра: %v", err)
	}
	m.update(func(s *Status) { *s = Status{State: StateDisconnected} })
}

func (m *Manager) update(fn func(*Status)) {
	m.mu.Lock()
	fn(&m.status)
	s := m.status
	m.mu.Unlock()
	m.onStatus(s)
}

// monitor раз в секунду считает скорость, меряет задержку через туннель
// (тем же способом, что и пинг в списке) и периодически узнаёт внешний IP.
func (m *Manager) monitor(ctx context.Context, c *core.Core) {
	proxyURL := &url.URL{Scheme: "http", Host: "127.0.0.1:" + strconv.Itoa(c.MixedPort)}
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL), MaxIdleConns: 4, IdleConnTimeout: time.Minute},
	}
	defer client.CloseIdleConnections()

	var (
		latencyBusy, ipBusy sync.Mutex
		nextIPCheck         atomic.Int64 // unix ns
		prevUp, prevDown    int64
	)
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	for {
		up, down := c.Traffic()
		m.updateIfCurrent(c, func(s *Status) {
			s.UpSpeed, s.DownSpeed = max(0, up-prevUp), max(0, down-prevDown)
			s.UpTotal, s.DownTotal = up, down
		})
		prevUp, prevDown = up, down

		if latencyBusy.TryLock() {
			go func() {
				defer latencyBusy.Unlock()
				lctx, cancel := context.WithTimeout(ctx, latencyTO)
				defer cancel()
				ms, err := c.Latency(lctx)
				if err != nil {
					ms = -1
				}
				m.updateIfCurrent(c, func(s *Status) { s.LatencyMs = ms })
			}()
		}

		if time.Now().UnixNano() > nextIPCheck.Load() && ipBusy.TryLock() {
			nextIPCheck.Store(time.Now().Add(ipInfoRetry).UnixNano())
			go func() {
				defer ipBusy.Unlock()
				info, err := fetchIPInfo(ctx, client)
				if err != nil {
					return
				}
				nextIPCheck.Store(time.Now().Add(ipInfoRefresh).UnixNano())
				m.updateIfCurrent(c, func(s *Status) {
					s.IP, s.CountryCode, s.City, s.Org = info.IP, info.CountryCode, info.City, info.Org
				})
			}()
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// updateIfCurrent обновляет статус, только если c всё ещё активное подключение:
// запоздавшие горутины старого подключения не должны затирать новое.
func (m *Manager) updateIfCurrent(c *core.Core, fn func(*Status)) {
	m.mu.Lock()
	if m.core != c {
		m.mu.Unlock()
		return
	}
	fn(&m.status)
	s := m.status
	m.mu.Unlock()
	m.onStatus(s)
}

type ipInfo struct {
	IP          string
	CountryCode string
	City        string
	Org         string
}

func fetchIPInfo(ctx context.Context, client *http.Client) (ipInfo, error) {
	var a struct {
		IP, Country, City, Org string
	}
	if err := getJSON(ctx, client, "https://ipinfo.io/json", &a); err == nil && a.IP != "" {
		return ipInfo{IP: a.IP, CountryCode: a.Country, City: a.City, Org: a.Org}, nil
	}
	var b struct {
		IP           string `json:"ip"`
		CountryCode  string `json:"country_code"`
		City         string `json:"city"`
		Organization string `json:"organization"`
	}
	if err := getJSON(ctx, client, "https://api.ip.sb/geoip", &b); err != nil {
		return ipInfo{}, err
	}
	return ipInfo{IP: b.IP, CountryCode: b.CountryCode, City: b.City, Org: b.Organization}, nil
}

func getJSON(ctx context.Context, client *http.Client, u string, v any) error {
	ctx, cancel := context.WithTimeout(ctx, ipInfoTO)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", u, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func friendlyError(err error, opts core.Options) error {
	msg := err.Error()
	if strings.Contains(msg, "address already in use") || strings.Contains(msg, "Only one usage of each socket address") {
		return fmt.Errorf("порт %d занят другой программой — укажите другой в настройках или 0 для автоматического выбора", opts.Port)
	}
	if opts.Mode == core.ModeTUN && (strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "access is denied") || strings.Contains(msg, "Access is denied")) {
		if runtime.GOOS == "windows" {
			return errors.New("для режима TUN запустите приложение от имени администратора")
		}
		exe, _ := os.Executable()
		return fmt.Errorf("для режима TUN нужны права: sudo setcap cap_net_admin,cap_net_raw,cap_net_bind_service+ep %s", exe)
	}
	return err
}
