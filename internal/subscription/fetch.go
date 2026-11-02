package subscription

import (
	"context"
	"fmt"
	"io"
	"net/http"
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

// Fetch скачивает тело подписки.
func Fetch(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("некорректный URL: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("не удалось загрузить подписку: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("сервер подписки ответил %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", fmt.Errorf("ошибка чтения подписки: %w", err)
	}
	return string(body), nil
}
