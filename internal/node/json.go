package node

import (
	"encoding/json"
	"fmt"
)

// ParseJSON понимает:
//   - полный конфиг xray или sing-box (объект с "outbounds");
//   - массив таких конфигов (JSON-подписки xray);
//   - отдельный outbound или массив outbound'ов.
func ParseJSON(data []byte) (nodes []Node, skipped int, err error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, 0, fmt.Errorf("некорректный JSON: %w", err)
	}
	p := &jsonParser{}
	p.walk(v, "")
	return uniqueIDs(p.nodes), p.skipped, nil
}

type jsonParser struct {
	nodes   []Node
	skipped int
}

func (p *jsonParser) walk(v any, remarks string) {
	switch t := v.(type) {
	case []any:
		for _, item := range t {
			p.walk(item, remarks)
		}
	case map[string]any:
		if outbounds, ok := t["outbounds"].([]any); ok {
			r, _ := t["remarks"].(string)
			for _, o := range outbounds {
				if m, ok := o.(map[string]any); ok {
					p.outbound(m, r)
				}
			}
			return
		}
		p.outbound(t, remarks)
	}
}

func (p *jsonParser) outbound(m map[string]any, remarks string) {
	raw, _ := json.Marshal(m)
	var (
		n   Node
		err error
	)
	switch {
	case m["type"] == "vless":
		n, err = fromSingbox(raw)
	case m["protocol"] == "vless":
		n, err = fromXray(raw)
	default:
		// direct, block, dns и прочие служебные outbound'ы не считаем пропущенными
		kind := m["type"]
		if kind == nil {
			kind = m["protocol"]
		}
		if kind != nil && !isService(kind) {
			p.skipped++
		}
		return
	}
	if err != nil || n.Server == "" || n.Port == 0 {
		p.skipped++
		return
	}
	if remarks != "" {
		n.Name = remarks
	}
	n.finalize()
	p.nodes = append(p.nodes, n)
}

func isService(v any) bool {
	switch v {
	case "direct", "block", "dns", "freedom", "blackhole", "selector", "urltest":
		return true
	}
	return false
}

type singboxOutbound struct {
	Tag        string `json:"tag"`
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
	UUID       string `json:"uuid"`
	Flow       string `json:"flow"`
	TLS        *struct {
		Enabled    bool     `json:"enabled"`
		ServerName string   `json:"server_name"`
		ALPN       []string `json:"alpn"`
		UTLS       *struct {
			Fingerprint string `json:"fingerprint"`
		} `json:"utls"`
		Reality *struct {
			Enabled   bool   `json:"enabled"`
			PublicKey string `json:"public_key"`
			ShortID   string `json:"short_id"`
		} `json:"reality"`
	} `json:"tls"`
	Transport *struct {
		Type        string         `json:"type"`
		Path        string         `json:"path"`
		Host        any            `json:"host"`
		Headers     map[string]any `json:"headers"`
		ServiceName string         `json:"service_name"`
	} `json:"transport"`
}

func fromSingbox(raw []byte) (Node, error) {
	var o singboxOutbound
	if err := json.Unmarshal(raw, &o); err != nil {
		return Node{}, err
	}
	n := Node{
		Name:   o.Tag,
		Server: o.Server,
		Port:   o.ServerPort,
		UUID:   o.UUID,
		Flow:   o.Flow,
	}
	if t := o.TLS; t != nil && t.Enabled {
		n.Security = "tls"
		n.SNI = t.ServerName
		n.ALPN = t.ALPN
		if t.UTLS != nil {
			n.Fingerprint = t.UTLS.Fingerprint
		}
		if r := t.Reality; r != nil && r.Enabled {
			n.Security = "reality"
			n.PublicKey = r.PublicKey
			n.ShortID = r.ShortID
		}
	}
	if t := o.Transport; t != nil {
		n.Network = t.Type
		n.Path = t.Path
		n.ServiceName = t.ServiceName
		n.Host = firstString(t.Host)
		if n.Host == "" {
			n.Host = firstString(t.Headers["Host"])
		}
	}
	return n, nil
}

type xrayUser struct {
	ID   string `json:"id"`
	Flow string `json:"flow"`
}

type xrayTransport struct {
	Path    string         `json:"path"`
	Host    string         `json:"host"`
	Headers map[string]any `json:"headers"`
}

type xrayOutbound struct {
	Tag      string `json:"tag"`
	Settings struct {
		Vnext []struct {
			Address string     `json:"address"`
			Port    int        `json:"port"`
			Users   []xrayUser `json:"users"`
		} `json:"vnext"`
		// упрощённый формат новых версий xray
		Address string `json:"address"`
		Port    int    `json:"port"`
		ID      string `json:"id"`
		Flow    string `json:"flow"`
	} `json:"settings"`
	StreamSettings *struct {
		Network     string `json:"network"`
		Security    string `json:"security"`
		TLSSettings *struct {
			ServerName  string   `json:"serverName"`
			Fingerprint string   `json:"fingerprint"`
			ALPN        []string `json:"alpn"`
		} `json:"tlsSettings"`
		RealitySettings *struct {
			ServerName  string `json:"serverName"`
			Fingerprint string `json:"fingerprint"`
			PublicKey   string `json:"publicKey"`
			ShortID     string `json:"shortId"`
		} `json:"realitySettings"`
		WSSettings          *xrayTransport `json:"wsSettings"`
		HTTPUpgradeSettings *xrayTransport `json:"httpupgradeSettings"`
		XHTTPSettings       *xrayTransport `json:"xhttpSettings"`
		GRPCSettings        *struct {
			ServiceName string `json:"serviceName"`
		} `json:"grpcSettings"`
	} `json:"streamSettings"`
}

func fromXray(raw []byte) (Node, error) {
	var o xrayOutbound
	if err := json.Unmarshal(raw, &o); err != nil {
		return Node{}, err
	}
	s := o.Settings
	n := Node{Name: o.Tag, Server: s.Address, Port: s.Port, UUID: s.ID, Flow: s.Flow}
	if len(s.Vnext) > 0 {
		v := s.Vnext[0]
		n.Server, n.Port = v.Address, v.Port
		if len(v.Users) > 0 {
			n.UUID, n.Flow = v.Users[0].ID, v.Users[0].Flow
		}
	}
	// тег "proxy" в xray-конфигах ничего не говорит о сервере
	if n.Name == "proxy" {
		n.Name = ""
	}

	ss := o.StreamSettings
	if ss == nil {
		return n, nil
	}
	n.Network = ss.Network
	n.Security = ss.Security
	if t := ss.TLSSettings; t != nil {
		n.SNI, n.Fingerprint, n.ALPN = t.ServerName, t.Fingerprint, t.ALPN
	}
	if r := ss.RealitySettings; r != nil {
		n.SNI, n.Fingerprint, n.PublicKey, n.ShortID = r.ServerName, r.Fingerprint, r.PublicKey, r.ShortID
	}
	for _, t := range []*xrayTransport{ss.WSSettings, ss.HTTPUpgradeSettings, ss.XHTTPSettings} {
		if t != nil {
			n.Path, n.Host = t.Path, t.Host
			if n.Host == "" {
				n.Host = firstString(t.Headers["Host"])
			}
		}
	}
	if ss.GRPCSettings != nil {
		n.ServiceName = ss.GRPCSettings.ServiceName
	}
	return n, nil
}

// firstString достаёт строку из значения, которое может быть строкой или массивом строк.
func firstString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		if len(t) > 0 {
			s, _ := t[0].(string)
			return s
		}
	}
	return ""
}
