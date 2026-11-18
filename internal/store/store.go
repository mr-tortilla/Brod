package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/mr-tortilla/brod/internal/node"
	"github.com/mr-tortilla/brod/internal/subscription"
)

const (
	SourceURL  = "url"  // подписка по ссылке, можно обновлять
	SourceText = "text" // вставленные vless:// ссылки
	SourceJSON = "json" // вставленный JSON-конфиг
)

type Profile struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Source    string      `json:"source"`
	URL       string      `json:"url,omitempty"`
	Nodes     []node.Node `json:"nodes"`
	Pinned    []string    `json:"pinned"` // ID закреплённых подключений в порядке закрепления
	UpdatedAt time.Time   `json:"updatedAt"`
	// Sub — срок и трафик подписки из ответа сервера; nil, если сервер их не сообщает.
	Sub *subscription.Info `json:"sub,omitempty"`
}

var ErrNotFound = errors.New("профиль не найден")

// Store хранит профили в одном JSON-файле.
type Store struct {
	mu       sync.Mutex
	path     string
	profiles []Profile
}

// DefaultPath — ~/.config/brod/profiles.json на Linux, %AppData%\brod\profiles.json на Windows.
// Данные из каталога прежнего названия (vless-client) переносятся при первом запуске.
func DefaultPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "brod")
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		if _, err := os.Stat(filepath.Join(base, "vless-client")); err == nil {
			_ = os.Rename(filepath.Join(base, "vless-client"), dir)
		}
	}
	return filepath.Join(dir, "profiles.json"), nil
}

func Open(path string) (*Store, error) {
	s := &Store{path: path}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.profiles); err != nil {
		return nil, err
	}
	return s, nil
}

// Dir — каталог, где лежит файл профилей; там же храним прочие служебные файлы.
func (s *Store) Dir() string {
	return filepath.Dir(s.path)
}

// List никогда не возвращает nil: во фронтенд nil-слайс приходит как null.
func (s *Store) List() []Profile {
	s.mu.Lock()
	defer s.mu.Unlock()
	list := append([]Profile{}, s.profiles...)
	for i := range list {
		if list[i].Pinned == nil {
			list[i].Pinned = []string{}
		}
	}
	return list
}

func (s *Store) Get(id string) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.index(id)
	if i < 0 {
		return Profile{}, ErrNotFound
	}
	return s.profiles[i], nil
}

// Save добавляет новый профиль или заменяет существующий с тем же ID.
func (s *Store) Save(p Profile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if i := s.index(p.ID); i >= 0 {
		s.profiles[i] = p
	} else {
		s.profiles = append(s.profiles, p)
	}
	return s.flush()
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.index(id)
	if i < 0 {
		return ErrNotFound
	}
	s.profiles = slices.Delete(s.profiles, i, i+1)
	return s.flush()
}

func (s *Store) index(id string) int {
	return slices.IndexFunc(s.profiles, func(p Profile) bool { return p.ID == id })
}

func (s *Store) flush() error {
	data, err := json.MarshalIndent(s.profiles, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(s.path, data)
}

// writeFile пишет файл атомарно: во временный файл, затем rename.
// Права 0600: в профилях лежат UUID — по сути пароли к серверам.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
