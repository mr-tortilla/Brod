package share

import (
	"strings"
	"testing"

	"github.com/mr-tortilla/brod/internal/node"
	"github.com/mr-tortilla/brod/internal/store"
)

func testNodes(t *testing.T) []node.Node {
	t.Helper()
	nodes, _, err := node.ParseText(strings.Join([]string{
		"vless://11111111-2222-3333-4444-555555555555@nl.example:443?type=tcp&security=reality&sni=www.google.com&fp=chrome&pbk=jNXHt1yRo0vDuchQlIP6Z0ZvjT3KtzVI-T4E7RoLJS0&sid=ab&flow=xtls-rprx-vision#NL",
		"vless://11111111-2222-3333-4444-555555555555@1.2.3.4:8443?type=ws&security=tls&path=%2Fws&host=h.example#DE",
		"vless://11111111-2222-3333-4444-555555555555@1.2.3.5:443?type=xhttp&security=tls#XHTTP",
	}, "\n"))
	if err != nil {
		t.Fatal(err)
	}
	return nodes
}

// JSON профиля импортируется обратно с теми же подключениями.
func TestProfileJSONRoundTrip(t *testing.T) {
	nodes := testNodes(t)
	d := Profile(store.Profile{Name: "test", Source: store.SourceText, Nodes: nodes})
	if d.Skipped != 1 { // xhttp не выражается в sing-box
		t.Fatalf("skipped = %d", d.Skipped)
	}
	back, _, err := node.ParseJSON([]byte(d.JSON))
	if err != nil {
		t.Fatal(err)
	}
	if len(back) != 2 || back[0].ID != nodes[0].ID || back[1].ID != nodes[1].ID {
		t.Fatalf("после импорта JSON: %+v", back)
	}
	if back[0].PublicKey != nodes[0].PublicKey || back[1].Path != "/ws" || back[1].Host != "h.example" {
		t.Fatalf("параметры потерялись: %+v", back)
	}
}

func TestProfileLinks(t *testing.T) {
	nodes := testNodes(t)
	d := Profile(store.Profile{Source: store.SourceText, Nodes: nodes})
	if d.LinkKind != KindLinks || strings.Count(d.Link, "vless://") != 3 {
		t.Fatalf("ссылки: %s", d.Link)
	}
	if d.QR == "" {
		t.Fatalf("QR не собрался: %s", d.QRError)
	}

	sub := Profile(store.Profile{Source: store.SourceURL, URL: "https://sub.example/abc", Nodes: nodes})
	if sub.LinkKind != KindSubscription || sub.Link != "https://sub.example/abc" {
		t.Fatalf("подписка: %+v", sub)
	}
}

func TestQRTooLarge(t *testing.T) {
	nodes := testNodes(t)
	many := make([]node.Node, 0, 60)
	for range 30 {
		many = append(many, nodes[:2]...)
	}
	d := Profile(store.Profile{Source: store.SourceText, Nodes: many})
	if d.QR != "" || d.QRError == "" {
		t.Fatal("для 60 ссылок QR должен не поместиться")
	}
}
