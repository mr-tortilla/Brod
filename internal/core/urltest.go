package core

import (
	"context"
	"encoding/json"
	"errors"
	stdlog "log"
	"sync"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	sjson "github.com/sagernet/sing/common/json"

	"github.com/mr-tortilla/brod/internal/node"
)

// Замер задержки: новое соединение через сервер + HEAD-запрос на testURL,
// то есть реальная задержка через туннель.

// Latency меряет задержку через активное подключение.
func (c *Core) Latency(ctx context.Context) (int, error) {
	out, ok := c.box.Outbound().Outbound(tagProxy)
	if !ok {
		return 0, errors.New("outbound не найден")
	}
	ms, err := urltest.URLTest(ctx, c.testURL, out)
	return max(1, int(ms)), err
}

// TestNodes меряет задержку через каждое подключение тем же способом, что и Latency.
// Для этого поднимается отдельный экземпляр sing-box без инбаундов, где каждое
// подключение — outbound с тегом, равным ID. report вызывается по мере готовности.
func TestNodes(ctx context.Context, nodes []node.Node, directDNS, testURL string, timeout time.Duration, concurrency int,
	report func(id string, ms int, err error)) error {

	outbounds := make([]any, 0, len(nodes)+1)
	testable := make([]string, 0, len(nodes))
	for _, n := range nodes {
		o, err := protocolOutbound(n)
		if err != nil {
			report(n.ID, -1, err)
			continue
		}
		o["tag"] = n.ID
		outbounds = append(outbounds, o)
		testable = append(testable, n.ID)
	}
	if len(testable) == 0 {
		return nil
	}
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": tagDirect})

	cfg := map[string]any{
		"log": map[string]any{"disabled": true},
		"dns": map[string]any{
			"servers": []any{map[string]any{"type": "udp", "tag": "local", "server": directDNS}},
		},
		"outbounds": outbounds,
		"route": map[string]any{
			"final":                   tagDirect,
			"auto_detect_interface":   true,
			"default_domain_resolver": "local",
		},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	boxCtx, cancel := context.WithCancel(include.Context(context.Background()))
	defer cancel()
	options, err := sjson.UnmarshalExtendedContext[option.Options](boxCtx, data)
	if err != nil {
		return err
	}
	instance, err := box.New(box.Options{Context: boxCtx, Options: options})
	if err != nil {
		return err
	}
	if err := instance.Start(); err != nil {
		return errors.Join(err, instance.Close())
	}
	defer func() {
		if err := instance.Close(); err != nil {
			stdlog.Printf("ошибка при остановке ядра проверки задержки: %v", err)
		}
	}()

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for _, id := range testable {
		out, _ := instance.Outbound().Outbound(id)
		wg.Go(func() {
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()

			tctx, tcancel := context.WithTimeout(ctx, timeout)
			defer tcancel()
			ms, err := urltest.URLTest(tctx, testURL, out)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				report(id, -1, err)
				return
			}
			report(id, max(1, int(ms)), nil)
		})
	}
	wg.Wait()
	return nil
}
