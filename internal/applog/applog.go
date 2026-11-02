// Package applog пишет лог приложения в файл: у GUI-приложения на Windows нет консоли,
// и без файла ошибки sing-box терялись бы.
package applog

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

const (
	FileName = "brod.log"
	maxSize  = 1 << 20 // при запуске лог больше 1 МБ уезжает в brod.log.1
)

// Init направляет стандартный log в dir/brod.log (и в stderr).
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, FileName)
	if fi, err := os.Stat(path); err == nil && fi.Size() > maxSize {
		_ = os.Rename(path, path+".1")
	}
	// Файл не закрывается намеренно: лог пишется всё время работы приложения.
	// Запись в os.File не буферизуется, при выходе файл закрывает ОС — данные не теряются.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	log.SetOutput(io.MultiWriter(os.Stderr, f))
	log.SetFlags(log.LstdFlags)
	return nil
}
