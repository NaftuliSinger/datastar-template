// Package crypto provides live cryptocurrency price updates from Coinbase.
package crypto

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// Coinbase Exchange public market data feed - free, no API key required.
// docs: https://docs.cdp.coinbase.com/exchange/websocket-feed/channels#ticker-channel
const feedURL = "wss://ws-feed.exchange.coinbase.com"

// DefaultProducts are the pairs the engine tracks
// For the template purpose we've put these 3 for now
var DefaultProducts = []string{"BTC-USD", "ETH-USD", "XRP-USD"}

// Ticker is a single price update for a product
type Ticker struct {
	ProductID string    `json:"product_id"`
	Price     string    `json:"price"`
	Open24h   string    `json:"open_24h"`
	High24h   string    `json:"high_24h"`
	Low24h    string    `json:"low_24h"`
	Volume24h string    `json:"volume_24h"`
	BestBid   string    `json:"best_bid"`
	BestAsk   string    `json:"best_ask"`
	Side      string    `json:"side"`
	Time      time.Time `json:"time"`
}

// Engine keeps a single websocket connection to the price feed and
// fans updates out to any number of subscribers (e.g. SSE handlers)
type Engine struct {
	products []string

	mu     sync.RWMutex
	latest map[string]Ticker
	subs   map[chan Ticker]struct{}
}

func NewEngine(products ...string) *Engine {
	if len(products) == 0 {
		products = DefaultProducts
	}
	return &Engine{
		products: products,
		latest:   make(map[string]Ticker),
		subs:     make(map[chan Ticker]struct{}),
	}
}

// Run connects to the feed and keeps reconnecting until ctx is cancelled.
// Call it in its own goroutine.
func (e *Engine) Run(ctx context.Context) {
	backoff := time.Second
	for {
		err := e.stream(ctx)
		if ctx.Err() != nil {
			return
		}
		log.Printf("crypto: feed disconnected: %v (reconnecting in %s)", err, backoff)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		// wait a bit longer after each failed attempt, up to 30s
		backoff = min(backoff*2, 30*time.Second)
	}
}

func (e *Engine) stream(ctx context.Context) error {
	conn, _, err := websocket.Dial(ctx, feedURL, nil)
	if err != nil {
		return err
	}
	defer conn.CloseNow()

	sub := map[string]any{
		"type":        "subscribe",
		"product_ids": e.products,
		"channels":    []string{"ticker"},
	}
	if err := wsjson.Write(ctx, conn, sub); err != nil {
		return err
	}
	log.Printf("crypto: subscribed to %v", e.products)

	for {
		var msg struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Reason  string `json:"reason"`
			Ticker
		}
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "ticker":
			e.publish(msg.Ticker)
		case "error":
			log.Printf("crypto: feed error: %s %s", msg.Message, msg.Reason)
		}
	}
}

func (e *Engine) publish(t Ticker) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.latest[t.ProductID] = t
	for ch := range e.subs {
		// never block the feed on a slow subscriber - drop the update instead
		select {
		case ch <- t:
		default:
		}
	}
}

// Subscribe returns a channel of live updates and a func to unsubscribe.
// Always call unsubscribe (e.g. defer it) when the client goes away.
func (e *Engine) Subscribe() (<-chan Ticker, func()) {
	ch := make(chan Ticker, 32)

	e.mu.Lock()
	e.subs[ch] = struct{}{}
	e.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			e.mu.Lock()
			delete(e.subs, ch)
			e.mu.Unlock()
			close(ch)
		})
	}
	return ch, unsubscribe
}

// Snapshot returns the latest known ticker for every product,
// handy for rendering the initial page before live updates arrive
func (e *Engine) Snapshot() map[string]Ticker {
	e.mu.RLock()
	defer e.mu.RUnlock()

	out := make(map[string]Ticker, len(e.latest))
	for k, v := range e.latest {
		out[k] = v
	}
	return out
}
