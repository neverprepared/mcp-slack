package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neverprepared/mcp-slack/internal/client"
	"github.com/slack-go/slack"
)

func RegisterPinTools(s *server.MCPServer, c *client.SlackClient) {
	s.AddTool(mcp.NewTool("pin_message",
		mcp.WithDescription("Pin a message to a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("timestamp", mcp.Required(), mcp.Description("Message ts.")),
	), wrap("pin_message", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		if err := bot.AddPin(strArg(req, "channel"), slack.ItemRef{
			Channel:   strArg(req, "channel"),
			Timestamp: strArg(req, "timestamp"),
		}); err != nil {
			return "", err
		}
		return okJSON(map[string]any{"pinned": strArg(req, "timestamp"), "channel": strArg(req, "channel")}), nil
	}))

	s.AddTool(mcp.NewTool("unpin_message",
		mcp.WithDescription("Unpin a message from a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("timestamp", mcp.Required(), mcp.Description("Message ts.")),
	), wrap("unpin_message", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		if err := bot.RemovePin(strArg(req, "channel"), slack.ItemRef{
			Channel:   strArg(req, "channel"),
			Timestamp: strArg(req, "timestamp"),
		}); err != nil {
			return "", err
		}
		return okJSON(map[string]any{"unpinned": strArg(req, "timestamp"), "channel": strArg(req, "channel")}), nil
	}))

	s.AddTool(mcp.NewTool("list_pins",
		mcp.WithDescription("List pinned items in a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
	), wrap("list_pins", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		items, _, err := bot.ListPins(strArg(req, "channel"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"items": items}), nil
	}))
}
