// Slack WebClient wrapper with two token modes:
//   - oauth: SLACK_BOT_TOKEN env var (xoxb-). Stable, long-lived.
//   - relay: browser-scraped xoxc-/xoxd- token from the encrypted cache,
//     refreshed live by the Ably subscriber.
//
// Mode is auto-selected: env wins if set, else cache.
// The bot client is property-guarded with a RWMutex; relay mode builds it
// lazily on first access from cache contents.
package client

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/neverprepared/mcp-slack/internal/cache"
	"github.com/slack-go/slack"
)

type SlackClient struct {
	mu         sync.RWMutex
	mode       string
	bot        *slack.Client
	user       *slack.Client
	tokenCache *cache.TokenCache
}

func New(tokenCache *cache.TokenCache) (*SlackClient, error) {
	c := &SlackClient{tokenCache: tokenCache}

	botToken := os.Getenv("SLACK_BOT_TOKEN")
	userToken := os.Getenv("SLACK_USER_TOKEN")

	if botToken != "" {
		c.mode = "oauth"
		c.bot = slack.New(botToken)
		if userToken != "" {
			c.user = slack.New(userToken)
		}
		log.Printf("slack client initialized (mode=oauth user_token=%v)", userToken != "")
		return c, nil
	}

	if tokenCache == nil {
		return nil, fmt.Errorf("no SLACK_BOT_TOKEN env and no token cache; set SLACK_BOT_TOKEN or run `mcp-slack setup`")
	}

	c.mode = "relay"
	if userToken != "" {
		c.user = slack.New(userToken)
	}
	log.Printf("slack client initialized (mode=relay)")
	return c, nil
}

func (c *SlackClient) Mode() string { return c.mode }

// Bot returns the bot-scoped Slack client, building it lazily in relay mode.
func (c *SlackClient) Bot() (*slack.Client, error) {
	c.mu.RLock()
	if c.bot != nil {
		b := c.bot
		c.mu.RUnlock()
		return b, nil
	}
	c.mu.RUnlock()

	// relay mode: build lazily from cache
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bot != nil {
		return c.bot, nil
	}
	payload, err := c.tokenCache.Get()
	if err != nil {
		return nil, err
	}
	if payload == nil {
		return nil, fmt.Errorf("no Slack token available yet; open app.slack.com with the Chrome extension installed or wait for the Ably relay")
	}
	b, err := buildBrowserClient(payload)
	if err != nil {
		return nil, err
	}
	c.bot = b
	return b, nil
}

// User returns the user-scoped Slack client.
//
// In oauth mode: requires SLACK_USER_TOKEN (xoxp-) — bots cannot call search.* or
// other user-only endpoints.
// In relay mode: the scraped xoxc- session token is already a user session, so
// User() falls back to Bot() rather than demanding a separate SLACK_USER_TOKEN.
func (c *SlackClient) User() (*slack.Client, error) {
	c.mu.RLock()
	u := c.user
	mode := c.mode
	c.mu.RUnlock()

	if u != nil {
		return u, nil
	}
	if mode == "relay" {
		return c.Bot()
	}
	return nil, fmt.Errorf("this endpoint requires SLACK_USER_TOKEN (xoxp-); Slack's search.* APIs and some user-scoped endpoints are unavailable to bots")
}

// RefreshFromToken swaps in a new bot client from a fresh relay payload.
// Called by the Ably subscriber on each received token.
func (c *SlackClient) RefreshFromToken(payload map[string]any) error {
	if c.mode != "relay" {
		return nil
	}
	b, err := buildBrowserClient(payload)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.bot = b
	c.mu.Unlock()
	log.Printf("slack bot client refreshed (team=%v user=%v)", payload["teamName"], payload["userName"])
	return nil
}

type cookieTransport struct {
	cookie string
	base   http.RoundTripper
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Cookie", "d="+t.cookie)
	return t.base.RoundTrip(req)
}

func buildBrowserClient(payload map[string]any) (*slack.Client, error) {
	token, _ := payload["token"].(string)
	if token == "" {
		return nil, fmt.Errorf("token payload missing 'token' field")
	}
	cookie, _ := payload["session"].(string)
	var opts []slack.Option
	if cookie != "" {
		opts = append(opts, slack.OptionHTTPClient(&http.Client{
			Transport: &cookieTransport{cookie: cookie, base: http.DefaultTransport},
		}))
	}
	return slack.New(token, opts...), nil
}
