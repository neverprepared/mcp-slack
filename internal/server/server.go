package server

import (
	"log"
	"os"
	"time"

	"github.com/mark3labs/mcp-go/server"
	mcpably "github.com/neverprepared/mcp-slack/internal/ably"
	"github.com/neverprepared/mcp-slack/internal/cache"
	"github.com/neverprepared/mcp-slack/internal/client"
	"github.com/neverprepared/mcp-slack/internal/tools"
)

type Server struct {
	MCP        *server.MCPServer
	Client     *client.SlackClient
	Subscriber *mcpably.Subscriber
}

func New(version string) (*Server, error) {
	mcp := server.NewMCPServer("mcp-slack", version,
		server.WithToolCapabilities(true),
	)
	tc := &cache.TokenCache{}
	c, err := client.New(tc)
	if err != nil {
		return nil, err
	}

	tools.RegisterMessageTools(mcp, c)
	tools.RegisterChannelTools(mcp, c)
	tools.RegisterUserTools(mcp, c)
	tools.RegisterFileTools(mcp, c)
	tools.RegisterSearchTools(mcp, c)
	tools.RegisterReactionTools(mcp, c)
	tools.RegisterPinTools(mcp, c)
	tools.RegisterMiscTools(mcp, c)

	var sub *mcpably.Subscriber
	if c.Mode() == "relay" {
		cfg, err := cache.LoadConfig()
		if err != nil {
			log.Printf("warn: could not load config: %v", err)
			cfg = map[string]any{}
		}
		channel, _ := cfg["ably_channel"].(string)
		if channel == "" {
			channel = os.Getenv("ABLY_CHANNEL")
		}
		if channel == "" {
			log.Printf("warn: relay mode but no ably_channel configured — tokens will only come from the on-disk cache; run `mcp-slack setup`")
		} else {
			sub = mcpably.New(channel, func(payload map[string]any) {
				if err := tc.Save(payload); err != nil {
					log.Printf("token cache save error: %v", err)
				}
				if err := c.RefreshFromToken(payload); err != nil {
					log.Printf("client refresh error: %v", err)
				}
			})
			sub.Start()
			// Best-effort wait so the first tool call has a token.
			if !sub.WaitReady(5 * time.Second) {
				log.Printf("warn: ably subscriber did not become ready within 5s; proceeding with cached token if available")
			}
		}
	}

	log.Printf("mcp-slack ready (mode=%s)", c.Mode())
	return &Server{MCP: mcp, Client: c, Subscriber: sub}, nil
}

func (s *Server) Stop() {
	if s.Subscriber != nil {
		s.Subscriber.Stop()
	}
}
