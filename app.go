package main

import (
	"context"
	"encoding/base64"
	"errors"
	"log"
	"net/url"
	"os"
	"os/exec"
	"reflect"
	goruntime "runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/mr-tortilla/brod/internal/core"
	"github.com/mr-tortilla/brod/internal/node"
	"github.com/mr-tortilla/brod/internal/share"
	"github.com/mr-tortilla/brod/internal/store"
	"github.com/mr-tortilla/brod/internal/subscription"
	"github.com/mr-tortilla/brod/internal/tray"
	"github.com/mr-tortilla/brod/internal/tunnel"
)

const (
	pingTimeout     = 6 * time.Second
	pingConcurrency = 12

	EventPingResult = "ping:result"
	EventPingDone   = "ping:done"
	EventStatus     = "tunnel:status"
)

// App — методы, доступные фронтенду.
type App struct {
	ctx    context.Context
	store  *store.Store
	tunnel *tunnel.Manager
	tray   *tray.Tray

	pingMu     sync.Mutex
	pingCancel map[string]context.CancelFunc // по ID профиля

	startupConnect *StartupAction // из аргументов запуска, отдаётся фронтенду один раз
}

// StartupAction — подключение, которое нужно выполнить сразу после запуска
// (после перезапуска с правами администратора).
type StartupAction struct {
	ProfileID string `json:"profileId"`
	NodeID    string `json:"nodeId"`
	Mode      string `json:"mode"`
}

// ErrNeedAdmin — фронтенд узнаёт по этому тексту, что нужно предложить перезапуск с правами.
const ErrNeedAdmin = "need-admin"

type CreateResult struct {
	Profile store.Profile `json:"profile"`
	Skipped int           `json:"skipped"` // записи других протоколов или с ошибками
}

type ConnectResult struct {
	Profile      store.Profile `json:"profile"`
	NodeID       string        `json:"nodeId"`                 // ID подключения в обновлённом профиле
	RefreshError string        `json:"refreshError,omitempty"` // подписку обновить не удалось, подключились по старым данным
}

type PingResult struct {
	ProfileID string `json:"profileId"`
	NodeID    string `json:"nodeId"`
	Ms        int    `json:"ms"` // -1 при ошибке
	Error     string `json:"error,omitempty"`
}

func NewApp(s *store.Store) *App {
	return &App{store: s, pingCancel: map[string]context.CancelFunc{}}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.tray = tray.Start(tray.Callbacks{
		Open:       a.showWindow,
		Disconnect: func() { a.tunnel.Disconnect() },
		Quit:       func() { runtime.Quit(ctx) },
	})
	a.tunnel = tunnel.NewManager(a.store.Dir(), func(st tunnel.Status) {
		runtime.EventsEmit(ctx, EventStatus, st)
		a.tray.Set(trayState(st))
	})
}

// shutdown обязательно отключает туннель: иначе в системе останется
// прописан прокси на уже закрытый порт.
func (a *App) shutdown(context.Context) {
	a.tunnel.Disconnect()
	a.tray.Stop()
}

func (a *App) showWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func trayState(st tunnel.Status) (tray.State, string) {
	switch {
	case st.State == tunnel.StateConnected && st.LatencyMs < 0:
		return tray.Lost, "Brod — нет связи: " + st.NodeName
	case st.State == tunnel.StateConnected:
		return tray.On, "Brod — подключено: " + st.NodeName
	case st.State == tunnel.StateConnecting:
		return tray.Off, "Brod — подключение: " + st.NodeName
	}
	return tray.Off, "Brod — отключено"
}

// SystemTheme — тема системы: "dark", "light" или "", если определить не удалось
// (тогда фронтенд полагается на prefers-color-scheme).
func (a *App) SystemTheme() string {
	return systemTheme()
}

// ApplyTheme подстраивает под тему фон окна (виден при изменении размера)
// и рамку окна на Windows.
func (a *App) ApplyTheme(dark bool) {
	if dark {
		runtime.WindowSetBackgroundColour(a.ctx, 17, 20, 26, 255)
		runtime.WindowSetDarkTheme(a.ctx)
	} else {
		runtime.WindowSetBackgroundColour(a.ctx, 244, 246, 250, 255)
		runtime.WindowSetLightTheme(a.ctx)
	}
}

// Version — версия приложения, подставленная при сборке.
func (a *App) Version() string {
	return version
}

func (a *App) GetStatus() tunnel.Status {
	return a.tunnel.Status()
}

// TakeStartupAction отдаёт отложенное подключение из аргументов запуска (один раз).
func (a *App) TakeStartupAction() *StartupAction {
	act := a.startupConnect
	a.startupConnect = nil
	return act
}

// RelaunchAsAdmin перезапускает Brod с правами администратора (Windows, окно UAC)
// и после запуска подключается к указанному подключению в режиме TUN.
func (a *App) RelaunchAsAdmin(profileID, nodeID string) error {
	err := relaunchElevated([]string{
		"--wait-pid", strconv.Itoa(os.Getpid()),
		"--connect", profileID + ":" + nodeID + ":" + string(core.ModeTUN),
	})
	if err != nil {
		// ERROR_CANCELLED: пользователь отказался в окне UAC
		if strings.Contains(err.Error(), "canceled") || strings.Contains(err.Error(), "cancelled") {
			return errors.New("перезапуск отменён")
		}
		return err
	}
	log.Printf("перезапуск с правами администратора")
	runtime.Quit(a.ctx)
	return nil
}

// OpenLogFolder открывает папку с логом в файловом менеджере.
func (a *App) OpenLogFolder() error {
	return openPath(a.store.Dir())
}

// Connect обновляет профиль-подписку и подключается к выбранному подключению.
// Если подписку обновить не удалось, подключается по сохранённым данным.
// mode — "proxy" или "tun" (строкой: Wails не генерирует TS-типы для строковых алиасов).
func (a *App) Connect(profileID, nodeID, modeName string) (ConnectResult, error) {
	mode := core.Mode(modeName)
	if mode != core.ModeProxy && mode != core.ModeTUN {
		return ConnectResult{}, errors.New("неизвестный режим подключения")
	}
	if mode == core.ModeTUN && !isElevated() {
		return ConnectResult{}, errors.New(ErrNeedAdmin)
	}
	// Отключаемся заранее, чтобы подписка скачивалась напрямую, а не через старый туннель.
	a.tunnel.Disconnect()

	p, err := a.store.Get(profileID)
	if err != nil {
		return ConnectResult{}, err
	}
	prev := slices.IndexFunc(p.Nodes, func(n node.Node) bool { return n.ID == nodeID })
	if prev < 0 {
		return ConnectResult{}, errors.New("подключение не найдено")
	}
	old := p.Nodes[prev]

	st, err := a.store.Settings()
	if err != nil {
		return ConnectResult{}, err
	}

	var res ConnectResult
	if p.Source == store.SourceURL && st.RefreshOnConnect {
		if updated, err := a.RefreshProfile(profileID); err != nil {
			res.RefreshError = err.Error()
		} else {
			p = updated.Profile
		}
	}
	res.Profile = p

	// Сначала по ID; если параметры поменялись (или ID старого формата) — по имени и адресу.
	i := slices.IndexFunc(p.Nodes, func(n node.Node) bool { return n.ID == nodeID })
	if i < 0 {
		i = slices.IndexFunc(p.Nodes, func(n node.Node) bool {
			return n.Name == old.Name && n.Server == old.Server && n.Port == old.Port
		})
	}
	if i < 0 {
		return res, errors.New("этого подключения больше нет в подписке, выберите другое")
	}
	res.NodeID = p.Nodes[i].ID
	if err := a.tunnel.Connect(p.ID, p.Nodes[i], coreOptions(st, mode)); err != nil {
		log.Printf("подключение к %q (%s) не удалось: %v", p.Nodes[i].Name, mode, err)
		return res, err
	}
	log.Printf("подключено к %q (%s)", p.Nodes[i].Name, mode)
	return res, nil
}

// coreOptions переводит настройки пользователя в параметры ядра.
func coreOptions(st store.Settings, mode core.Mode) core.Options {
	bypass, _ := core.ParseBypass(st.Bypass)
	strategy := st.IPv6
	if strategy == "off" {
		strategy = "ipv4_only"
	}
	return core.Options{
		Mode:             mode,
		Bypass:           bypass,
		DNS:              core.DNS{Direct: st.DirectDNS, Remote: st.RemoteDNS},
		Port:             st.MixedPort,
		AllowLAN:         st.AllowLAN,
		BypassLAN:        st.BypassLAN,
		DNSStrategy:      strategy,
		SystemProxyInTUN: st.SystemProxyInTUN,
		TunStack:         st.TunStack,
		TunMTU:           st.TunMTU,
		StrictRoute:      st.StrictRoute,
		TestURL:          st.TestURL,
	}
}

func (a *App) GetSettings() (store.Settings, error) {
	return a.store.Settings()
}

type SaveSettingsResult struct {
	Invalid     []string `json:"invalid"`     // строки исключений, которые не удалось разобрать
	Reconnected bool     `json:"reconnected"` // активное подключение перезапущено с новыми правилами
}

func (a *App) DefaultSettings() store.Settings {
	return store.DefaultSettings()
}

// SaveSettings сохраняет настройки и, если изменилось что-то, влияющее на подключение,
// переподключается, чтобы изменения сразу заработали. Смена темы переподключения не вызывает.
func (a *App) SaveSettings(st store.Settings) (SaveSettingsResult, error) {
	_, invalid := core.ParseBypass(st.Bypass)
	res := SaveSettingsResult{Invalid: invalid}
	if invalid == nil {
		res.Invalid = []string{}
	}
	prev, err := a.store.Settings()
	if err != nil {
		return res, err
	}
	if err := a.store.SaveSettings(st); err != nil {
		return res, err
	}
	saved, err := a.store.Settings()
	if err != nil {
		return res, err
	}

	cur := a.tunnel.Status()
	if cur.State != tunnel.StateConnected ||
		reflect.DeepEqual(coreOptions(prev, cur.Mode), coreOptions(saved, cur.Mode)) {
		return res, nil
	}
	p, err := a.store.Get(cur.ProfileID)
	if err != nil {
		return res, err
	}
	i := slices.IndexFunc(p.Nodes, func(n node.Node) bool { return n.ID == cur.NodeID })
	if i < 0 {
		return res, errors.New("активное подключение не найдено, переподключитесь вручную")
	}
	if err := a.tunnel.Connect(p.ID, p.Nodes[i], coreOptions(saved, cur.Mode)); err != nil {
		return res, err
	}
	res.Reconnected = true
	return res, nil
}

func (a *App) Disconnect() {
	a.tunnel.Disconnect()
}

func (a *App) ListProfiles() []store.Profile {
	return a.store.List()
}

// CreateProfileFromURL принимает URL подписки или одну/несколько vless:// ссылок.
func (a *App) CreateProfileFromURL(name, input string) (CreateResult, error) {
	input = strings.TrimSpace(input)
	p := store.Profile{ID: node.NewID(), Name: strings.TrimSpace(name), Source: store.SourceText}

	text := input
	if isHTTP(input) {
		p.Source, p.URL = store.SourceURL, input
		body, info, err := subscription.Fetch(a.ctx, input)
		if err != nil {
			return CreateResult{}, err
		}
		text, p.Sub = body, info
		if p.Name == "" {
			u, _ := url.Parse(input)
			p.Name = u.Hostname()
		}
	}

	nodes, skipped, err := node.ParseText(text)
	if err != nil {
		return CreateResult{}, err
	}
	return a.create(p, nodes, skipped)
}

// CreateProfileFromJSON принимает вставленный конфиг: JSON (xray, sing-box) или YAML (Clash, Mihomo).
func (a *App) CreateProfileFromJSON(name, input string) (CreateResult, error) {
	nodes, skipped, err := node.ParseText(input)
	if err != nil {
		return CreateResult{}, err
	}
	p := store.Profile{ID: node.NewID(), Name: strings.TrimSpace(name), Source: store.SourceJSON}
	return a.create(p, nodes, skipped)
}

func (a *App) create(p store.Profile, nodes []node.Node, skipped int) (CreateResult, error) {
	if len(nodes) == 0 {
		return CreateResult{}, errors.New("не найдено ни одного поддерживаемого подключения (VLESS, Hysteria2 или TUIC)")
	}
	if p.Name == "" {
		p.Name = "Профиль " + time.Now().Format("02.01 15:04")
	}
	p.Pinned = carryPinned(p.Pinned, p.Nodes, nodes)
	p.Nodes = nodes
	p.UpdatedAt = time.Now()
	if err := a.store.Save(p); err != nil {
		return CreateResult{}, err
	}
	return CreateResult{Profile: p, Skipped: skipped}, nil
}

// carryPinned переносит закрепления на обновлённый список подключений: по ID, а если ID
// изменился — по имени и адресу. Пропавшие подключения из закреплений убираются.
func carryPinned(pinned []string, oldNodes, newNodes []node.Node) []string {
	out := []string{}
	for _, id := range pinned {
		if slices.ContainsFunc(newNodes, func(n node.Node) bool { return n.ID == id }) {
			out = append(out, id)
			continue
		}
		i := slices.IndexFunc(oldNodes, func(n node.Node) bool { return n.ID == id })
		if i < 0 {
			continue
		}
		old := oldNodes[i]
		j := slices.IndexFunc(newNodes, func(n node.Node) bool {
			return n.Name == old.Name && n.Server == old.Server && n.Port == old.Port
		})
		if j >= 0 && !slices.Contains(out, newNodes[j].ID) {
			out = append(out, newNodes[j].ID)
		}
	}
	return out
}

// RefreshProfile заново скачивает подписку.
func (a *App) RefreshProfile(id string) (CreateResult, error) {
	p, err := a.store.Get(id)
	if err != nil {
		return CreateResult{}, err
	}
	if p.Source != store.SourceURL {
		return CreateResult{}, errors.New("обновлять можно только профили-подписки")
	}
	body, info, err := subscription.Fetch(a.ctx, p.URL)
	if err != nil {
		return CreateResult{}, err
	}
	if info != nil {
		p.Sub = info
	}
	nodes, skipped, err := node.ParseText(body)
	if err != nil {
		return CreateResult{}, err
	}
	return a.create(p, nodes, skipped)
}

// ShareProfile — профиль целиком: ссылка (URL подписки или vless:// ссылки), QR, JSON.
func (a *App) ShareProfile(profileID string) (share.Data, error) {
	p, err := a.store.Get(profileID)
	if err != nil {
		return share.Data{}, err
	}
	return share.Profile(p), nil
}

// ShareNode — одно подключение: vless:// ссылка, QR с ней, JSON.
func (a *App) ShareNode(profileID, nodeID string) (share.Data, error) {
	p, err := a.store.Get(profileID)
	if err != nil {
		return share.Data{}, err
	}
	i := slices.IndexFunc(p.Nodes, func(n node.Node) bool { return n.ID == nodeID })
	if i < 0 {
		return share.Data{}, errors.New("подключение не найдено")
	}
	return share.Node(p.Nodes[i]), nil
}

// SaveFile сохраняет текст или data:-URL (например, PNG с QR) через системный диалог.
// Возвращает путь или пустую строку, если пользователь отменил сохранение.
func (a *App) SaveFile(defaultName, content string) (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{DefaultFilename: defaultName})
	if err != nil || path == "" {
		return "", err
	}
	data := []byte(content)
	if rest, ok := strings.CutPrefix(content, "data:"); ok {
		_, b64, found := strings.Cut(rest, ";base64,")
		if !found {
			return "", errors.New("неподдерживаемый формат данных")
		}
		if data, err = base64.StdEncoding.DecodeString(b64); err != nil {
			return "", err
		}
	}
	return path, os.WriteFile(path, data, 0o600)
}

// SetPinned закрепляет подключение в начале списка или открепляет его.
func (a *App) SetPinned(profileID, nodeID string, pinned bool) (store.Profile, error) {
	p, err := a.store.Get(profileID)
	if err != nil {
		return p, err
	}
	p.Pinned = slices.DeleteFunc(slices.Clone(p.Pinned), func(id string) bool { return id == nodeID })
	if pinned {
		p.Pinned = append(p.Pinned, nodeID)
	}
	if p.Pinned == nil {
		p.Pinned = []string{}
	}
	return p, a.store.Save(p)
}

func (a *App) DeleteProfile(id string) error {
	if a.tunnel.Status().ProfileID == id {
		a.tunnel.Disconnect()
	}
	return a.store.Delete(id)
}

// PingProfile запускает проверку всех подключений профиля в фоне: реальную задержку
// через каждый сервер, тем же способом, что и в панели статуса.
// Результаты приходят событиями ping:result по мере готовности, в конце — ping:done.
// Новый запуск для того же профиля отменяет предыдущий.
func (a *App) PingProfile(id string) error {
	p, err := a.store.Get(id)
	if err != nil {
		return err
	}
	st, err := a.store.Settings()
	if err != nil {
		return err
	}

	a.pingMu.Lock()
	if cancel := a.pingCancel[p.ID]; cancel != nil {
		cancel()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.pingCancel[p.ID] = cancel
	a.pingMu.Unlock()

	go func() {
		defer cancel()
		err := core.TestNodes(ctx, p.Nodes, st.DirectDNS, st.TestURL, pingTimeout, pingConcurrency, func(id string, ms int, err error) {
			res := PingResult{ProfileID: p.ID, NodeID: id, Ms: ms}
			if err != nil {
				res.Ms, res.Error = -1, shortError(err)
			}
			runtime.EventsEmit(a.ctx, EventPingResult, res)
		})
		if err != nil {
			// ядро не запустилось — помечаем все подключения ошибкой, чтобы не висело «…»
			for _, n := range p.Nodes {
				runtime.EventsEmit(a.ctx, EventPingResult, PingResult{ProfileID: p.ID, NodeID: n.ID, Ms: -1, Error: err.Error()})
			}
		}
		if ctx.Err() == nil {
			runtime.EventsEmit(a.ctx, EventPingDone, p.ID)
		}
	}()
	return nil
}

func isHTTP(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func shortError(err error) string {
	msg := err.Error()
	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(msg, "i/o timeout"):
		return "таймаут"
	case strings.Contains(msg, "connection refused"):
		return "отказ в соединении"
	case strings.Contains(msg, "no such host"):
		return "домен не найден"
	case strings.Contains(msg, "network is unreachable"):
		return "сеть недоступна"
	case strings.Contains(msg, "EOF") || strings.Contains(msg, "reset by peer"):
		return "сервер оборвал соединение"
	}
	return msg
}

// openPath открывает файл или папку программой по умолчанию.
func openPath(path string) error {
	switch goruntime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	}
	return exec.Command("xdg-open", path).Start()
}
