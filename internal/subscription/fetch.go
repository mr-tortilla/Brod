package subscription

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const maxBody = 10 << 20

// UserAgent задаёт main с учётом версии приложения.
var UserAgent = "Brod"

// Подписка всегда грузится напрямую: прокси из переменных окружения (HTTP_PROXY и т.п.)
// не используется, чтобы не зависеть от других VPN-клиентов, которые их прописывают.
var client = &http.Client{
	Timeout:   20 * time.Second,
	Transport: &http.Transport{Proxy: nil, ForceAttemptHTTP2: true},
}

// Info — сведения о подписке из заголовка subscription-userinfo
// (его отдают Marzban, Remnawave, 3x-ui и большинство других панелей). Нули — нет данных:
// Total = 0 — трафик не ограничен, Expire = 0 — бессрочно.
type Info struct {
	Upload   int64 `json:"upload"`   // байт
	Download int64 `json:"download"` // байт
	Total    int64 `json:"total"`    // байт
	Expire   int64 `json:"expire"`   // unix-время окончания
}

// Fetch скачивает тело подписки и сведения о ней (nil, если сервер их не прислал).
func Fetch(ctx context.Context, url string) (string, *Info, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("некорректный URL: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("не удалось загрузить подписку: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("сервер подписки ответил %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", nil, fmt.Errorf("ошибка чтения подписки: %w", err)
	}
	return string(body), parseUserInfo(resp.Header.Get("Subscription-Userinfo")), nil
}

// parseUserInfo разбирает «upload=0; download=123; total=456; expire=1797157422».
// Некоторые панели присылают числа с дробной частью — их тоже понимаем.
func parseUserInfo(h string) *Info {
	if strings.TrimSpace(h) == "" {
		return nil
	}
	var info Info
	found := false
	for _, part := range strings.Split(h, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			continue
		}
		n := int64(f)
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "upload":
			info.Upload = n
		case "download":
			info.Download = n
		case "total":
			info.Total = n
		case "expire":
			info.Expire = n
		default:
			continue
		}
		found = true
	}
	if !found {
		return nil
	}
	return &info
}
