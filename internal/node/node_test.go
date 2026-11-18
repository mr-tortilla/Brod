package node

import (
	"encoding/base64"
	"strings"
	"testing"
)

const realityLink = "vless://11111111-2222-3333-4444-555555555555@example.com:443" +
	"?type=tcp&security=reality&sni=www.google.com&fp=chrome&pbk=PUBKEY&sid=ab12&flow=xtls-rprx-vision#%F0%9F%87%B3%F0%9F%87%B1%20NL"

func TestParseLink(t *testing.T) {
	n, err := ParseLink(realityLink)
	if err != nil {
		t.Fatal(err)
	}
	want := Node{
		Name: "🇳🇱 NL", Server: "example.com", Port: 443, UUID: "11111111-2222-3333-4444-555555555555",
		Flow: "xtls-rprx-vision", Network: "tcp", Security: "reality", SNI: "www.google.com",
		Fingerprint: "chrome", PublicKey: "PUBKEY", ShortID: "ab12",
	}
	want.ID = n.ID
	if n.Name != want.Name || n.Server != want.Server || n.Port != want.Port || n.UUID != want.UUID ||
		n.Flow != want.Flow || n.Security != want.Security || n.PublicKey != want.PublicKey || n.ShortID != want.ShortID {
		t.Fatalf("got %+v\nwant %+v", n, want)
	}
}

func TestParseTextBase64(t *testing.T) {
	body := realityLink + "\nvmess://abc\nvless://u@[2001:db8::1]:8443?type=ws&security=tls&path=%2Fws&host=h.example\n"
	nodes, skipped, err := ParseText(base64.StdEncoding.EncodeToString([]byte(body)))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 || skipped != 1 {
		t.Fatalf("nodes=%d skipped=%d", len(nodes), skipped)
	}
	if ws := nodes[1]; ws.Server != "2001:db8::1" || ws.Network != "ws" || ws.Path != "/ws" || ws.Host != "h.example" {
		t.Fatalf("ws node: %+v", ws)
	}
}

func TestParseJSONXray(t *testing.T) {
	cfg := `[{"remarks":"Germany","outbounds":[
		{"protocol":"vless","tag":"proxy","settings":{"vnext":[{"address":"de.example","port":443,"users":[{"id":"u1","flow":"xtls-rprx-vision"}]}]},
		 "streamSettings":{"network":"tcp","security":"reality","realitySettings":{"serverName":"sni.de","publicKey":"pk","shortId":"01"}}},
		{"protocol":"freedom","tag":"direct"}]}]`
	nodes, skipped, err := ParseJSON([]byte(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || skipped != 0 {
		t.Fatalf("nodes=%d skipped=%d", len(nodes), skipped)
	}
	n := nodes[0]
	if n.Name != "Germany" || n.Server != "de.example" || n.UUID != "u1" || n.Security != "reality" || n.SNI != "sni.de" {
		t.Fatalf("%+v", n)
	}
}

func TestParseJSONSingbox(t *testing.T) {
	cfg := `{"outbounds":[
		{"type":"vless","tag":"fi-1","server":"fi.example","server_port":8443,"uuid":"u2",
		 "tls":{"enabled":true,"server_name":"fi.example","utls":{"enabled":true,"fingerprint":"firefox"}},
		 "transport":{"type":"grpc","service_name":"grpc-svc"}},
		{"type":"trojan","server":"x","server_port":1},
		{"type":"direct","tag":"direct"}]}`
	nodes, skipped, err := ParseJSON([]byte(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || skipped != 1 {
		t.Fatalf("nodes=%d skipped=%d", len(nodes), skipped)
	}
	n := nodes[0]
	if n.Name != "fi-1" || n.Port != 8443 || n.Security != "tls" || n.Fingerprint != "firefox" || n.Network != "grpc" || n.ServiceName != "grpc-svc" {
		t.Fatalf("%+v", n)
	}
}

// Провайдеры могут менять SNI/shortId при каждой выдаче подписки — ID от этого меняться не должен.
func TestIDStableAcrossMasking(t *testing.T) {
	a, _ := ParseLink("vless://u@192.0.2.4:443?security=reality&sni=a.example&sid=01&pbk=k1#NL")
	b, _ := ParseLink("vless://u@192.0.2.4:443?security=reality&sni=b.example&sid=02&pbk=k2#NL")
	if a.ID != b.ID {
		t.Fatalf("ID изменился: %s != %s", a.ID, b.ID)
	}
	c, _ := ParseLink("vless://u@192.0.2.4:443?security=reality&sni=a.example#DE")
	if a.ID == c.ID {
		t.Fatal("разные подключения получили одинаковый ID")
	}
}

func TestLinkRoundTrip(t *testing.T) {
	cases := []string{
		realityLink,
		"vless://u@[2001:db8::1]:8443?type=ws&security=tls&path=%2Fws%3Fed%3D2048&host=h.example&alpn=h2,http/1.1#WS%20узел",
		"vless://u@192.0.2.4:443?type=grpc&security=tls&serviceName=svc&fp=firefox#gRPC",
	}
	for _, link := range cases {
		a, err := ParseLink(link)
		if err != nil {
			t.Fatal(err)
		}
		b, err := ParseLink(Link(a))
		if err != nil {
			t.Fatalf("%s: %v", Link(a), err)
		}
		if a.ID != b.ID || a.SNI != b.SNI || a.PublicKey != b.PublicKey || a.ShortID != b.ShortID ||
			a.Path != b.Path || a.Host != b.Host || a.ServiceName != b.ServiceName || a.Fingerprint != b.Fingerprint ||
			strings.Join(a.ALPN, ",") != strings.Join(b.ALPN, ",") || a.Flow != b.Flow {
			t.Fatalf("после круга по ссылке:\n%+v\n%+v\n%s", a, b, Link(a))
		}
	}
}

func TestHysteria2AndTUICLinkRoundTrip(t *testing.T) {
	links := []string{
		"hysteria2://fake-password@hy2.example:443?sni=hy2.example&insecure=1&obfs=salamander&obfs-password=fake-obfs&upmbps=50&downmbps=100&alpn=h3#Hy2",
		"tuic://11111111-2222-3333-4444-555555555555:fake-password@tuic.example:443?sni=tuic.example&allow_insecure=1&congestion_control=bbr&udp_relay_mode=native&alpn=h3#TUIC",
	}
	for _, link := range links {
		a, err := ParseLink(link)
		if err != nil {
			t.Fatal(err)
		}
		b, err := ParseLink(Link(a))
		if err != nil {
			t.Fatalf("%s: %v", Link(a), err)
		}
		if a.ID != b.ID || a.Protocol != b.Protocol || a.Password != b.Password || a.UUID != b.UUID ||
			a.SNI != b.SNI || a.Insecure != b.Insecure || a.ObfsType != b.ObfsType || a.ObfsPassword != b.ObfsPassword ||
			a.UpMbps != b.UpMbps || a.DownMbps != b.DownMbps || a.CongestionControl != b.CongestionControl ||
			a.UDPRelayMode != b.UDPRelayMode || strings.Join(a.ALPN, ",") != strings.Join(b.ALPN, ",") {
			t.Fatalf("параметры потерялись после round trip:\n%+v\n%+v", a, b)
		}
	}
}

func TestParseSingboxHysteria2AndTUIC(t *testing.T) {
	cfg := `{"outbounds":[
		{"type":"hysteria2","tag":"hy2-test","server":"hy2.example","server_port":443,"password":"fake-password","up_mbps":50,"down_mbps":100,"obfs":{"type":"salamander","password":"fake-obfs"},"tls":{"enabled":true,"server_name":"hy2.example","insecure":true,"alpn":["h3"]}},
		{"type":"tuic","tag":"tuic-test","server":"tuic.example","server_port":443,"uuid":"11111111-2222-3333-4444-555555555555","password":"fake-password","congestion_control":"bbr","udp_relay_mode":"native","tls":{"enabled":true,"server_name":"tuic.example","alpn":["h3"]}}
	]}`
	nodes, skipped, err := ParseJSON([]byte(cfg))
	if err != nil || skipped != 0 || len(nodes) != 2 {
		t.Fatalf("nodes=%d skipped=%d err=%v", len(nodes), skipped, err)
	}
	if nodes[0].Protocol != "hysteria2" || nodes[0].ObfsType != "salamander" || nodes[0].UpMbps != 50 || !nodes[0].Insecure {
		t.Fatalf("Hysteria2: %+v", nodes[0])
	}
	if nodes[1].Protocol != "tuic" || nodes[1].CongestionControl != "bbr" || nodes[1].UDPRelayMode != "native" {
		t.Fatalf("TUIC: %+v", nodes[1])
	}
}
