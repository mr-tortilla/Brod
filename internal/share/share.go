// Package share готовит профиль или отдельное подключение к передаче:
// ссылкой, QR-кодом и JSON.
package share

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/skip2/go-qrcode"

	"github.com/mr-tortilla/brod/internal/core"
	"github.com/mr-tortilla/brod/internal/node"
	"github.com/mr-tortilla/brod/internal/store"
)

const qrSize = 512

type LinkKind string

const (
	KindSubscription LinkKind = "subscription" // URL подписки
	KindLinks        LinkKind = "links"        // vless:// ссылки построчно
)

type Data struct {
	Title    string   `json:"title"`
	Link     string   `json:"link"`
	LinkKind LinkKind `json:"linkKind"`
	JSON     string   `json:"json"`
	QR       string   `json:"qr"`                // data:image/png;base64,…
	QRError  string   `json:"qrError,omitempty"` // почему QR не получился
	Skipped  int      `json:"skipped"`           // подключения, которые нельзя выразить в JSON sing-box
}

// Profile: для подписки ссылка и QR — её URL (так профиль будет обновляться у получателя),
// для остальных — vless:// ссылки всех подключений. JSON — outbound'ы sing-box.
func Profile(p store.Profile) Data {
	d := Data{Title: p.Name}
	if p.Source == store.SourceURL {
		d.Link, d.LinkKind = p.URL, KindSubscription
	} else {
		links := make([]string, len(p.Nodes))
		for i, n := range p.Nodes {
			links[i] = node.Link(n)
		}
		d.Link, d.LinkKind = strings.Join(links, "\n"), KindLinks
	}
	d.JSON, d.Skipped = outboundsJSON(p.Nodes)
	d.QR, d.QRError = qr(d.Link)
	return d
}

// Node: ссылка vless:// и QR с ней — их понимают все клиенты, включая мобильные;
// JSON — один outbound sing-box.
func Node(n node.Node) Data {
	d := Data{Title: n.Name, Link: node.Link(n), LinkKind: KindLinks}
	d.JSON, d.Skipped = outboundsJSON([]node.Node{n})
	d.QR, d.QRError = qr(d.Link)
	return d
}

// outboundsJSON — конфиг sing-box из одних outbound'ов: его импортирует и Brod, и клиенты на sing-box.
func outboundsJSON(nodes []node.Node) (string, int) {
	outbounds := make([]any, 0, len(nodes))
	tags := map[string]int{}
	skipped := 0
	for _, n := range nodes {
		o, err := core.Outbound(n)
		if err != nil {
			skipped++
			continue
		}
		// теги в sing-box должны быть уникальными
		tag := o["tag"].(string)
		if tags[tag]++; tags[tag] > 1 {
			o["tag"] = fmt.Sprintf("%s (%d)", tag, tags[tag])
		}
		outbounds = append(outbounds, o)
	}
	data, _ := json.MarshalIndent(map[string]any{"outbounds": outbounds}, "", "  ")
	return string(data), skipped
}

func qr(content string) (string, string) {
	if content == "" {
		return "", "нечего закодировать"
	}
	// для длинного содержимого — минимальная коррекция ошибок, чтобы влезло больше
	level := qrcode.Medium
	if len(content) > 300 {
		level = qrcode.Low
	}
	png, err := qrcode.Encode(content, level, qrSize)
	if err != nil {
		return "", "слишком много данных для QR-кода — поделитесь ссылкой или JSON"
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), ""
}
