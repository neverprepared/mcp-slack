// Background Ably subscriber: fetches history on start, then subscribes live.
// Runs in a goroutine. On each decrypted message, calls the supplied onToken
// callback (which writes to TokenCache and refreshes the SlackClient).
package ably

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/ably/ably-go/ably"
	mcrypto "github.com/neverprepared/mcp-slack/internal/crypto"
	"github.com/neverprepared/mcp-slack/internal/secrets"
)

type TokenCallback func(payload map[string]any)

type Subscriber struct {
	channelName string
	onToken     TokenCallback
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	ready       chan struct{}
	readyOnce   sync.Once
}

func New(channelName string, onToken TokenCallback) *Subscriber {
	return &Subscriber{
		channelName: channelName,
		onToken:     onToken,
		ready:       make(chan struct{}),
	}
}

func (s *Subscriber) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run(ctx)
	}()
}

func (s *Subscriber) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

// WaitReady blocks until the initial history fetch completes (or timeout).
func (s *Subscriber) WaitReady(timeout time.Duration) bool {
	select {
	case <-s.ready:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (s *Subscriber) markReady() {
	s.readyOnce.Do(func() { close(s.ready) })
}

func (s *Subscriber) run(ctx context.Context) {
	apiKey, err := secrets.GetAblyKey()
	if err != nil {
		log.Printf("ably: %v", err)
		s.markReady()
		return
	}
	passphrase, err := secrets.GetPassphrase()
	if err != nil {
		log.Printf("ably: %v", err)
		s.markReady()
		return
	}

	s.fetchHistory(ctx, apiKey, passphrase)
	s.markReady()

	if ctx.Err() != nil {
		return
	}
	s.subscribe(ctx, apiKey, passphrase)
}

func (s *Subscriber) fetchHistory(ctx context.Context, apiKey, passphrase string) {
	rest, err := ably.NewREST(ably.WithKey(apiKey))
	if err != nil {
		log.Printf("ably history: create REST client: %v", err)
		return
	}
	ch := rest.Channels.Get(s.channelName)
	pages, err := ch.History().Pages(ctx)
	if err != nil {
		log.Printf("ably history: %v", err)
		return
	}
	if pages.Next(ctx) {
		items := pages.Items()
		if len(items) > 0 {
			s.handle(items[0].Data, passphrase)
		}
	} else {
		log.Printf("ably history: channel %q is empty", s.channelName)
	}
}

func (s *Subscriber) subscribe(ctx context.Context, apiKey, passphrase string) {
	rt, err := ably.NewRealtime(ably.WithKey(apiKey))
	if err != nil {
		log.Printf("ably subscribe: create realtime client: %v", err)
		return
	}
	defer rt.Close()

	ch := rt.Channels.Get(s.channelName)
	_, err = ch.SubscribeAll(ctx, func(msg *ably.Message) {
		s.handle(msg.Data, passphrase)
	})
	if err != nil {
		log.Printf("ably subscribe: %v", err)
		return
	}
	log.Printf("ably subscribed to %q", s.channelName)
	<-ctx.Done()
}

func (s *Subscriber) handle(data any, passphrase string) {
	str, ok := data.(string)
	if !ok {
		log.Printf("ably: message data is not a string; skipping")
		return
	}
	plaintext, err := mcrypto.Decrypt(str, passphrase)
	if err != nil {
		log.Printf("ably: decrypt failed: %v", err)
		return
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		log.Printf("ably: unmarshal failed: %v", err)
		return
	}
	if payload["token"] == nil {
		log.Printf("ably: payload missing 'token'; skipping")
		return
	}
	s.onToken(payload)
}
