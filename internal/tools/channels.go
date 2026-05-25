package tools

import (
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neverprepared/mcp-slack/internal/client"
	"github.com/slack-go/slack"
)

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func RegisterChannelTools(s *server.MCPServer, c *client.SlackClient) {
	s.AddTool(mcp.NewTool("list_channels",
		mcp.WithDescription("List channels (conversations) in the workspace."),
		mcp.WithString("types", mcp.Description("Comma-separated channel types: public_channel, private_channel, mpim, im (default public_channel).")),
		mcp.WithBoolean("exclude_archived", mcp.Description("Skip archived channels (default true).")),
		mcp.WithNumber("limit", mcp.Description("Max channels per page (default 200, max 1000).")),
		mcp.WithString("cursor", mcp.Description("Pagination cursor.")),
	), wrap("list_channels", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		params := slack.GetConversationsParameters{
			Types:          []string{strDefault(req, "types", "public_channel")},
			ExcludeArchived: boolDefault(req, "exclude_archived", true),
			Limit:          intDefault(req, "limit", 200),
		}
		if cur, ok := optStr(req, "cursor"); ok {
			params.Cursor = cur
		}
		channels, next, err := bot.GetConversations(&params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"channels": channels, "next_cursor": next}), nil
	}))

	s.AddTool(mcp.NewTool("get_channel_info",
		mcp.WithDescription("Get metadata for a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithBoolean("include_num_members", mcp.Description("Include member count (default true).")),
	), wrap("get_channel_info", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		ch, err := bot.GetConversationInfo(&slack.GetConversationInfoInput{
			ChannelID:         strArg(req, "channel"),
			IncludeNumMembers: boolDefault(req, "include_num_members", true),
		})
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"channel": ch}), nil
	}))

	s.AddTool(mcp.NewTool("get_channel_history",
		mcp.WithDescription("Fetch message history of a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithNumber("limit", mcp.Description("Max messages (default 100).")),
		mcp.WithString("cursor", mcp.Description("Pagination cursor.")),
		mcp.WithString("oldest", mcp.Description("Earliest ts to include.")),
		mcp.WithString("latest", mcp.Description("Latest ts to include.")),
		mcp.WithBoolean("inclusive", mcp.Description("Include messages exactly at oldest/latest boundaries.")),
	), wrap("get_channel_history", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		params := slack.GetConversationHistoryParameters{
			ChannelID: strArg(req, "channel"),
			Limit:     intDefault(req, "limit", 100),
			Inclusive: boolDefault(req, "inclusive", false),
		}
		if cur, ok := optStr(req, "cursor"); ok {
			params.Cursor = cur
		}
		if v, ok := optStr(req, "oldest"); ok {
			params.Oldest = v
		}
		if v, ok := optStr(req, "latest"); ok {
			params.Latest = v
		}
		resp, err := bot.GetConversationHistory(&params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{
			"messages":    resp.Messages,
			"has_more":    resp.HasMore,
			"next_cursor": resp.ResponseMetaData.NextCursor,
		}), nil
	}))

	s.AddTool(mcp.NewTool("create_channel",
		mcp.WithDescription("Create a new channel."),
		mcp.WithString("name", mcp.Required(), mcp.Description("Channel name (lowercase, no spaces, max 80 chars).")),
		mcp.WithBoolean("is_private", mcp.Description("Create as private channel (default false).")),
	), wrap("create_channel", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		ch, err := bot.CreateConversation(slack.CreateConversationParams{
			ChannelName: strArg(req, "name"),
			IsPrivate:   boolDefault(req, "is_private", false),
		})
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"channel": ch}), nil
	}))

	s.AddTool(mcp.NewTool("archive_channel",
		mcp.WithDescription("Archive a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
	), wrap("archive_channel", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		if err := bot.ArchiveConversation(strArg(req, "channel")); err != nil {
			return "", err
		}
		return okJSON(map[string]any{"archived": strArg(req, "channel")}), nil
	}))

	s.AddTool(mcp.NewTool("unarchive_channel",
		mcp.WithDescription("Unarchive a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
	), wrap("unarchive_channel", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		if err := bot.UnArchiveConversation(strArg(req, "channel")); err != nil {
			return "", err
		}
		return okJSON(map[string]any{"unarchived": strArg(req, "channel")}), nil
	}))

	s.AddTool(mcp.NewTool("rename_channel",
		mcp.WithDescription("Rename a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("name", mcp.Required(), mcp.Description("New name (lowercase, no spaces).")),
	), wrap("rename_channel", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		ch, err := bot.RenameConversation(strArg(req, "channel"), strArg(req, "name"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"channel": ch}), nil
	}))

	s.AddTool(mcp.NewTool("set_channel_topic",
		mcp.WithDescription("Set the topic of a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("topic", mcp.Required(), mcp.Description("New topic text.")),
	), wrap("set_channel_topic", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		topic, err := bot.SetTopicOfConversation(strArg(req, "channel"), strArg(req, "topic"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"topic": topic}), nil
	}))

	s.AddTool(mcp.NewTool("set_channel_purpose",
		mcp.WithDescription("Set the purpose (description) of a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("purpose", mcp.Required(), mcp.Description("New purpose text.")),
	), wrap("set_channel_purpose", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		purpose, err := bot.SetPurposeOfConversation(strArg(req, "channel"), strArg(req, "purpose"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"purpose": purpose}), nil
	}))

	s.AddTool(mcp.NewTool("join_channel",
		mcp.WithDescription("Join a public channel as the bot."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
	), wrap("join_channel", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		ch, _, _, err := bot.JoinConversation(strArg(req, "channel"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"channel": ch}), nil
	}))

	s.AddTool(mcp.NewTool("leave_channel",
		mcp.WithDescription("Leave a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
	), wrap("leave_channel", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		if _, err := bot.LeaveConversation(strArg(req, "channel")); err != nil {
			return "", err
		}
		return okJSON(map[string]any{"left": strArg(req, "channel")}), nil
	}))

	s.AddTool(mcp.NewTool("invite_to_channel",
		mcp.WithDescription("Invite users to a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("users", mcp.Required(), mcp.Description("Comma-separated user IDs (e.g. \"U123,U456\").")),
	), wrap("invite_to_channel", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		ids := splitCSV(strArg(req, "users"))
		ch, err := bot.InviteUsersToConversation(strArg(req, "channel"), ids...)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"channel": ch}), nil
	}))

	s.AddTool(mcp.NewTool("kick_from_channel",
		mcp.WithDescription("Remove a user from a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("user", mcp.Required(), mcp.Description("User ID.")),
	), wrap("kick_from_channel", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		if err := bot.KickUserFromConversation(strArg(req, "channel"), strArg(req, "user")); err != nil {
			return "", err
		}
		return okJSON(map[string]any{"kicked": strArg(req, "user"), "channel": strArg(req, "channel")}), nil
	}))

	s.AddTool(mcp.NewTool("list_channel_members",
		mcp.WithDescription("List user IDs that are members of a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithNumber("limit", mcp.Description("Max members per page (default 200).")),
		mcp.WithString("cursor", mcp.Description("Pagination cursor.")),
	), wrap("list_channel_members", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		params := slack.GetUsersInConversationParameters{
			ChannelID: strArg(req, "channel"),
			Limit:     intDefault(req, "limit", 200),
		}
		if cur, ok := optStr(req, "cursor"); ok {
			params.Cursor = cur
		}
		members, next, err := bot.GetUsersInConversation(&params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"members": members, "next_cursor": next}), nil
	}))

	s.AddTool(mcp.NewTool("open_dm",
		mcp.WithDescription("Open or resume a direct message / multi-party DM."),
		mcp.WithString("users", mcp.Required(), mcp.Description("Comma-separated user IDs (1 = DM, 2-8 = MPIM).")),
	), wrap("open_dm", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		ch, _, _, err := bot.OpenConversation(&slack.OpenConversationParameters{
			Users: splitCSV(strArg(req, "users")),
		})
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"channel": ch}), nil
	}))

	s.AddTool(mcp.NewTool("close_dm",
		mcp.WithDescription("Close a DM or MPIM channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
	), wrap("close_dm", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		_, _, err = bot.CloseConversation(strArg(req, "channel"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"closed": strArg(req, "channel")}), nil
	}))
}
