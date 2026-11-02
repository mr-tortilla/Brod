package node

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

// Node — одно VLESS-подключение, приведённое к единому виду
// независимо от источника (ссылка, xray JSON, sing-box JSON).
type Node struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Server      string   `json:"server"`
	Port        int      `json:"port"`
	UUID        string   `json:"uuid"`
	Flow        string   `json:"flow,omitempty"`
	Network     string   `json:"network"`  // tcp, ws, grpc, http, httpupgrade, xhttp
	Security    string   `json:"security"` // none, tls, reality
	SNI         string   `json:"sni,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	PublicKey   string   `json:"publicKey,omitempty"`
	ShortID     string   `json:"shortId,omitempty"`
	ALPN        []string `json:"alpn,omitempty"`
	Path        string   `json:"path,omitempty"`
	Host        string   `json:"host,omitempty"`
	ServiceName string   `json:"serviceName,omitempty"`
}

// finalize проставляет дефолты, имя, если его нет, и ID.
// ID вычисляется из того, что определяет сервер, чтобы переживать обновление подписки.
// Параметры маскировки (SNI, shortId, ключ, отпечаток, путь, host) в ID не входят:
// некоторые провайдеры генерируют их заново при каждой выдаче подписки.
func (n *Node) finalize() {
	if n.Network == "" {
		n.Network = "tcp"
	}
	if n.Security == "" {
		n.Security = "none"
	}
	if n.Name == "" {
		n.Name = n.Server + ":" + strconv.Itoa(n.Port)
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{
		n.Name, n.Server, strconv.Itoa(n.Port), n.UUID, n.Network, n.Security,
	}, "\x00")))
	n.ID = hex.EncodeToString(sum[:8])
}

// uniqueIDs разводит ID полностью одинаковых записей.
func uniqueIDs(nodes []Node) []Node {
	seen := make(map[string]int, len(nodes))
	for i := range nodes {
		id := nodes[i].ID
		seen[id]++
		if seen[id] > 1 {
			nodes[i].ID = id + "-" + strconv.Itoa(seen[id])
		}
	}
	return nodes
}

func NewID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
