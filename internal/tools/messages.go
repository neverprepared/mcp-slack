package tools

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neverprepared/mcp-slack/internal/client"
	"github.com/slack-go/slack"
)

func RegisterMessageTools(s *server.MCPServer, c *client.SlackClient) {
	s.AddTool(mcp.NewTool("post_message",
		mcp.WithDescription("Post a message to a channel, DM, or thread."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID (C…), user ID for DM (U…), or channel name (#general).")),
		mcp.WithString("text", mcp.Description("Plain-text message; fallback when blocks are used.")),
		mcp.WithString("thread_ts", mcp.Description("Parent message ts to reply in a thread (e.g. \"1699999999.123456\").")),
		mcp.WithString("blocks_json", mcp.Description("JSON string of Block Kit blocks array.")),
		mcp.WithString("attachments_json", mcp.Description("JSON string of legacy attachments array.")),
		mcp.WithBoolean("reply_broadcast", mcp.Description("Surface thread reply to the channel (default false).")),
		mcp.WithBoolean("unfurl_links", mcp.Description("Auto-unfurl URLs (default true).")),
		mcp.WithBoolean("unfurl_media", mcp.Description("Auto-unfurl media (default true).")),
	), wrap("post_message", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		opts := []slack.MsgOption{slack.MsgOptionText(strDefault(req, "text", ""), false)}
		if ts, ok := optStr(req, "thread_ts"); ok {
			opts = append(opts, slack.MsgOptionTS(ts))
		}
		if bj, ok := optStr(req, "blocks_json"); ok {
			blocks, err := parseBlocks(bj)
			if err != nil {
				return "", fmt.Errorf("blocks_json: %w", err)
			}
			opts = append(opts, slack.MsgOptionBlocks(blocks...))
		}
		if aj, ok := optStr(req, "attachments_json"); ok {
			var att []slack.Attachment
			if err := json.Unmarshal([]byte(aj), &att); err != nil {
				return "", fmt.Errorf("attachments_json: %w", err)
			}
			opts = append(opts, slack.MsgOptionAttachments(att...))
		}
		if boolDefault(req, "reply_broadcast", false) {
			opts = append(opts, slack.MsgOptionBroadcast())
		}
		if !boolDefault(req, "unfurl_links", true) {
			opts = append(opts, slack.MsgOptionDisableLinkUnfurl())
		}
		if !boolDefault(req, "unfurl_media", true) {
			opts = append(opts, slack.MsgOptionDisableMediaUnfurl())
		}
		ch, ts, err := bot.PostMessage(strArg(req, "channel"), opts...)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"ts": ts, "channel": ch}), nil
	}))

	s.AddTool(mcp.NewTool("update_message",
		mcp.WithDescription("Update (edit) an existing message."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID containing the message.")),
		mcp.WithString("ts", mcp.Required(), mcp.Description("Timestamp of the message to edit.")),
		mcp.WithString("text", mcp.Description("New text content.")),
		mcp.WithString("blocks_json", mcp.Description("JSON string of replacement Block Kit blocks array.")),
		mcp.WithString("attachments_json", mcp.Description("JSON string of replacement attachments array.")),
	), wrap("update_message", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		opts := []slack.MsgOption{slack.MsgOptionText(strDefault(req, "text", ""), false)}
		if bj, ok := optStr(req, "blocks_json"); ok {
			blocks, err := parseBlocks(bj)
			if err != nil {
				return "", fmt.Errorf("blocks_json: %w", err)
			}
			opts = append(opts, slack.MsgOptionBlocks(blocks...))
		}
		if aj, ok := optStr(req, "attachments_json"); ok {
			var att []slack.Attachment
			if err := json.Unmarshal([]byte(aj), &att); err != nil {
				return "", fmt.Errorf("attachments_json: %w", err)
			}
			opts = append(opts, slack.MsgOptionAttachments(att...))
		}
		ch, ts, _, err := bot.UpdateMessage(strArg(req, "channel"), strArg(req, "ts"), opts...)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"ts": ts, "channel": ch}), nil
	}))

	s.AddTool(mcp.NewTool("delete_message",
		mcp.WithDescription("Delete a message."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID containing the message.")),
		mcp.WithString("ts", mcp.Required(), mcp.Description("Timestamp of the message.")),
	), wrap("delete_message", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		ch, ts, err := bot.DeleteMessage(strArg(req, "channel"), strArg(req, "ts"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"channel": ch, "ts": ts}), nil
	}))

	s.AddTool(mcp.NewTool("post_ephemeral",
		mcp.WithDescription("Post an ephemeral message visible only to a single user in a channel."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("user", mcp.Required(), mcp.Description("User ID to show the message to.")),
		mcp.WithString("text", mcp.Required(), mcp.Description("Message text.")),
		mcp.WithString("blocks_json", mcp.Description("Optional JSON string of Block Kit blocks array.")),
	), wrap("post_ephemeral", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		opts := []slack.MsgOption{slack.MsgOptionText(strArg(req, "text"), false)}
		if bj, ok := optStr(req, "blocks_json"); ok {
			blocks, err := parseBlocks(bj)
			if err != nil {
				return "", fmt.Errorf("blocks_json: %w", err)
			}
			opts = append(opts, slack.MsgOptionBlocks(blocks...))
		}
		ts, err := bot.PostEphemeral(strArg(req, "channel"), strArg(req, "user"), opts...)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"message_ts": ts}), nil
	}))

	s.AddTool(mcp.NewTool("schedule_message",
		mcp.WithDescription("Schedule a message to be posted in the future."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("text", mcp.Required(), mcp.Description("Message text.")),
		mcp.WithNumber("post_at", mcp.Required(), mcp.Description("Unix epoch seconds at which to post (must be <120 days out).")),
		mcp.WithString("thread_ts", mcp.Description("Optional thread parent ts.")),
	), wrap("schedule_message", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		postAt := fmt.Sprintf("%d", intDefault(req, "post_at", 0))
		opts := []slack.MsgOption{slack.MsgOptionText(strArg(req, "text"), false)}
		if ts, ok := optStr(req, "thread_ts"); ok {
			opts = append(opts, slack.MsgOptionTS(ts))
		}
		scheduledID, ch, err := bot.ScheduleMessage(strArg(req, "channel"), postAt, opts...)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{
			"scheduled_message_id": scheduledID,
			"channel":              ch,
			"post_at":              postAt,
		}), nil
	}))

	s.AddTool(mcp.NewTool("list_scheduled_messages",
		mcp.WithDescription("List pending scheduled messages."),
		mcp.WithString("channel", mcp.Description("Filter to a specific channel ID (optional).")),
		mcp.WithNumber("limit", mcp.Description("Max results (default 100).")),
	), wrap("list_scheduled_messages", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		params := &slack.GetScheduledMessagesParameters{
			Limit: intDefault(req, "limit", 100),
		}
		if ch, ok := optStr(req, "channel"); ok {
			params.Channel = ch
		}
		msgs, _, err := bot.GetScheduledMessages(params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"scheduled_messages": msgs}), nil
	}))

	s.AddTool(mcp.NewTool("delete_scheduled_message",
		mcp.WithDescription("Cancel a pending scheduled message."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID the message was scheduled to.")),
		mcp.WithString("scheduled_message_id", mcp.Required(), mcp.Description("ID returned from schedule_message.")),
	), wrap("delete_scheduled_message", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		id := strArg(req, "scheduled_message_id")
		_, err = bot.DeleteScheduledMessage(&slack.DeleteScheduledMessageParameters{
			Channel:            strArg(req, "channel"),
			ScheduledMessageID: id,
		})
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"deleted": id}), nil
	}))

	s.AddTool(mcp.NewTool("get_permalink",
		mcp.WithDescription("Get a permanent URL to a message."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("message_ts", mcp.Required(), mcp.Description("Message timestamp.")),
	), wrap("get_permalink", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		link, err := bot.GetPermalink(&slack.PermalinkParameters{
			Channel: strArg(req, "channel"),
			Ts:      strArg(req, "message_ts"),
		})
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"permalink": link}), nil
	}))

	s.AddTool(mcp.NewTool("get_thread_replies",
		mcp.WithDescription("Fetch replies to a threaded message."),
		mcp.WithString("channel", mcp.Required(), mcp.Description("Channel ID.")),
		mcp.WithString("ts", mcp.Required(), mcp.Description("Parent message ts.")),
		mcp.WithNumber("limit", mcp.Description("Max replies (default 100).")),
		mcp.WithString("cursor", mcp.Description("Pagination cursor from a prior call.")),
	), wrap("get_thread_replies", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		params := slack.GetConversationRepliesParameters{
			ChannelID: strArg(req, "channel"),
			Timestamp: strArg(req, "ts"),
			Limit:     intDefault(req, "limit", 100),
		}
		if cur, ok := optStr(req, "cursor"); ok {
			params.Cursor = cur
		}
		msgs, hasMore, nextCursor, err := bot.GetConversationReplies(&params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{
			"messages":    msgs,
			"has_more":    hasMore,
			"next_cursor": nextCursor,
		}), nil
	}))
}

// parseBlocks wraps a JSON array string in a {"blocks":[...]} envelope so
// slack-go's Blocks.UnmarshalJSON can dispatch to the correct concrete types.
func parseBlocks(blocksJSON string) ([]slack.Block, error) {
	var wrapper slack.Blocks
	if err := json.Unmarshal([]byte(`{"blocks":`+blocksJSON+`}`), &wrapper); err != nil {
		return nil, err
	}
	return wrapper.BlockSet, nil
}
