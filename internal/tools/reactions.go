package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neverprepared/mcp-slack/internal/client"
	"github.com/slack-go/slack"
)

func RegisterReactionTools(s *server.MCPServer, c *client.SlackClient) {
	s.AddTool(mcp.NewTool("add_reaction",
		mcp.WithDescription("Add an emoji reaction to a message."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("timestamp", mcp.Required(), mcp.Description("Message ts.")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Emoji name without colons (e.g. \"thumbsup\").")),
	), wrap("add_reaction", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		err = bot.AddReaction(strArg(req, "name"), slack.ItemRef{
			Channel:   strArg(req, "channel"),
			Timestamp: strArg(req, "timestamp"),
		})
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"added": strArg(req, "name")}), nil
	}))

	s.AddTool(mcp.NewTool("remove_reaction",
		mcp.WithDescription("Remove an emoji reaction from a message."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("timestamp", mcp.Required(), mcp.Description("Message ts.")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Emoji name without colons.")),
	), wrap("remove_reaction", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		err = bot.RemoveReaction(strArg(req, "name"), slack.ItemRef{
			Channel:   strArg(req, "channel"),
			Timestamp: strArg(req, "timestamp"),
		})
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"removed": strArg(req, "name")}), nil
	}))

	s.AddTool(mcp.NewTool("get_reactions",
		mcp.WithDescription("Get reactions on a message."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("timestamp", mcp.Required(), mcp.Description("Message ts.")),
		mcp.WithBoolean("full", mcp.Description("Return full reactor lists (default true).")),
	), wrap("get_reactions", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		resp, err := bot.GetReactions(slack.ItemRef{
			Channel:   strArg(req, "channel"),
			Timestamp: strArg(req, "timestamp"),
		}, slack.GetReactionsParameters{
			Full: boolDefault(req, "full", true),
		})
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"message": resp}), nil
	}))

	s.AddTool(mcp.NewTool("list_user_reactions",
		mcp.WithDescription("List items a user has reacted to."),
		mcp.WithString("user", mcp.Description("User ID (omit for the authenticated user).")),
		mcp.WithNumber("count", mcp.Description("Items per page (default 100).")),
		mcp.WithNumber("page", mcp.Description("Page number (default 1).")),
		mcp.WithBoolean("full", mcp.Description("Include full reactor lists (default false).")),
	), wrap("list_user_reactions", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		params := slack.ListReactionsParameters{
			Count: intDefault(req, "count", 100),
			Page:  intDefault(req, "page", 1),
			Full:  boolDefault(req, "full", false),
		}
		if u, ok := optStr(req, "user"); ok {
			params.User = u
		}
		reactions, paging, err := bot.ListReactions(params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"items": reactions, "paging": paging}), nil
	}))
}
