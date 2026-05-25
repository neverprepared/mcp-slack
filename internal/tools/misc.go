package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neverprepared/mcp-slack/internal/client"
	"github.com/slack-go/slack"
)

func RegisterMiscTools(s *server.MCPServer, c *client.SlackClient) {
	s.AddTool(mcp.NewTool("auth_test",
		mcp.WithDescription("Verify the bot token and return the authed identity."),
	), wrap("auth_test", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		resp, err := bot.AuthTest()
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{
			"user":    resp.User,
			"user_id": resp.UserID,
			"team":    resp.Team,
			"team_id": resp.TeamID,
			"bot_id":  resp.BotID,
			"url":     resp.URL,
		}), nil
	}))

	s.AddTool(mcp.NewTool("get_team_info",
		mcp.WithDescription("Get info about the current workspace."),
	), wrap("get_team_info", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		info, err := bot.GetTeamInfo()
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"team": info}), nil
	}))

	s.AddTool(mcp.NewTool("list_emoji",
		mcp.WithDescription("List all custom emoji in the workspace."),
	), wrap("list_emoji", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		emoji, err := bot.GetEmoji()
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"emoji": emoji}), nil
	}))

	s.AddTool(mcp.NewTool("list_bookmarks",
		mcp.WithDescription("List bookmarks for a channel."),
		mcp.WithString("channel_id", mcp.Required(), mcp.Description("Channel ID.")),
	), wrap("list_bookmarks", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		bookmarks, err := bot.ListBookmarks(strArg(req, "channel_id"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"bookmarks": bookmarks}), nil
	}))

	s.AddTool(mcp.NewTool("add_bookmark",
		mcp.WithDescription("Add a bookmark to a channel."),
		mcp.WithString("channel_id", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Bookmark title.")),
		mcp.WithString("type", mcp.Description("Bookmark type (default \"link\").")),
		mcp.WithString("link", mcp.Description("URL for the bookmark (required for type=link).")),
		mcp.WithString("emoji", mcp.Description("Optional emoji shortcode (e.g. \":book:\").")),
	), wrap("add_bookmark", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		params := slack.AddBookmarkParameters{
			Title: strArg(req, "title"),
			Type:  strDefault(req, "type", "link"),
		}
		if v, ok := optStr(req, "link"); ok {
			params.Link = v
		}
		if v, ok := optStr(req, "emoji"); ok {
			params.Emoji = v
		}
		bookmark, err := bot.AddBookmark(strArg(req, "channel_id"), params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"bookmark": bookmark}), nil
	}))

	s.AddTool(mcp.NewTool("edit_bookmark",
		mcp.WithDescription("Edit an existing channel bookmark."),
		mcp.WithString("channel_id", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("bookmark_id", mcp.Required(), mcp.Description("Bookmark ID.")),
		mcp.WithString("title", mcp.Description("New title.")),
		mcp.WithString("link", mcp.Description("New URL.")),
		mcp.WithString("emoji", mcp.Description("New emoji shortcode.")),
	), wrap("edit_bookmark", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		params := slack.EditBookmarkParameters{}
		if v, ok := optStr(req, "title"); ok {
			params.Title = &v
		}
		if v, ok := optStr(req, "link"); ok {
			params.Link = v
		}
		if v, ok := optStr(req, "emoji"); ok {
			params.Emoji = &v
		}
		bookmark, err := bot.EditBookmark(strArg(req, "channel_id"), strArg(req, "bookmark_id"), params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"bookmark": bookmark}), nil
	}))

	s.AddTool(mcp.NewTool("remove_bookmark",
		mcp.WithDescription("Remove a channel bookmark."),
		mcp.WithString("channel_id", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("bookmark_id", mcp.Required(), mcp.Description("Bookmark ID.")),
	), wrap("remove_bookmark", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		id := strArg(req, "bookmark_id")
		if err := bot.RemoveBookmark(strArg(req, "channel_id"), id); err != nil {
			return "", err
		}
		return okJSON(map[string]any{"removed": id}), nil
	}))
}
