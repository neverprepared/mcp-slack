package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neverprepared/mcp-slack/internal/client"
	"github.com/slack-go/slack"
)

func RegisterUserTools(s *server.MCPServer, c *client.SlackClient) {
	s.AddTool(mcp.NewTool("list_users",
		mcp.WithDescription("List users in the workspace. Returns up to limit users; call again to page (slack-go fetches all internally, so cursor is not needed)."),
		mcp.WithNumber("limit", mcp.Description("Max users to return (default 200).")),
	), wrap("list_users", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		limit := intDefault(req, "limit", 200)
		members, err := bot.GetUsersContext(context.Background(), slack.GetUsersOptionLimit(limit))
		if err != nil {
			return "", err
		}
		if len(members) > limit {
			members = members[:limit]
		}
		return okJSON(map[string]any{"members": members}), nil
	}))

	s.AddTool(mcp.NewTool("get_user_info",
		mcp.WithDescription("Get full info for a user."),
		mcp.WithString("user", mcp.Required(), mcp.Description("User ID (U…).")),
		mcp.WithBoolean("include_locale", mcp.Description("Include locale info (default false).")),
	), wrap("get_user_info", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		user, err := bot.GetUserInfo(strArg(req, "user"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"user": user}), nil
	}))

	s.AddTool(mcp.NewTool("lookup_user_by_email",
		mcp.WithDescription("Find a user by email address."),
		mcp.WithString("email", mcp.Required(), mcp.Description("Email to look up.")),
	), wrap("lookup_user_by_email", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		user, err := bot.GetUserByEmail(strArg(req, "email"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"user": user}), nil
	}))

	s.AddTool(mcp.NewTool("search_users",
		mcp.WithDescription("Search users by name, display name, real name, or email. Slack has no native users.search, so this paginates users.list and filters locally (case-insensitive substring match)."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Substring to match.")),
		mcp.WithNumber("limit", mcp.Description("Max matches to return (default 200).")),
		mcp.WithString("match_fields", mcp.Description("Comma-separated profile fields to search (default: name,real_name,display_name,email).")),
	), wrap("search_users", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		query := strings.ToLower(strArg(req, "query"))
		limit := intDefault(req, "limit", 200)
		fields := strings.Split(strDefault(req, "match_fields", "name,real_name,display_name,email"), ",")

		allMembers, err := bot.GetUsersContext(context.Background())
		if err != nil {
			return "", err
		}
		var matches []slack.User
		for _, m := range allMembers {
			if m.Deleted {
				continue
			}
			if matchesUser(m, query, fields) {
				matches = append(matches, m)
				if len(matches) >= limit {
					return okJSON(map[string]any{"matches": matches, "truncated": true}), nil
				}
			}
		}
		return okJSON(map[string]any{"matches": matches, "truncated": false}), nil
	}))

	s.AddTool(mcp.NewTool("get_user_presence",
		mcp.WithDescription("Get a user's presence (active/away)."),
		mcp.WithString("user", mcp.Required(), mcp.Description("User ID.")),
	), wrap("get_user_presence", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		presence, err := bot.GetUserPresence(strArg(req, "user"))
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"presence": presence.Presence, "online": presence.Online}), nil
	}))

	s.AddTool(mcp.NewTool("get_user_profile",
		mcp.WithDescription("Get a user's full profile (including custom fields)."),
		mcp.WithString("user", mcp.Required(), mcp.Description("User ID.")),
		mcp.WithBoolean("include_labels", mcp.Description("Include custom field labels (default false).")),
	), wrap("get_user_profile", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		profile, err := bot.GetUserProfile(&slack.GetUserProfileParameters{
			UserID:        strArg(req, "user"),
			IncludeLabels: boolDefault(req, "include_labels", false),
		})
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"profile": profile}), nil
	}))

	s.AddTool(mcp.NewTool("set_user_profile",
		mcp.WithDescription("Update a user's real name or status. Requires SLACK_USER_TOKEN (xoxp-). For name=\"real_name\" sets real name; for name=\"status_text\" sets custom status (pair with status_emoji)."),
		mcp.WithString("user", mcp.Description("User ID to update (admin only; omit to update self).")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Field to set: \"real_name\", \"status_text\", or \"status_emoji\".")),
		mcp.WithString("value", mcp.Required(), mcp.Description("New value for the field.")),
	), wrap("set_user_profile", func(req mcp.CallToolRequest) (string, error) {
		userClient, err := c.User()
		if err != nil {
			return "", err
		}
		userID, _ := optStr(req, "user")
		name := strArg(req, "name")
		value := strArg(req, "value")
		switch name {
		case "real_name":
			if err := userClient.SetUserRealNameContextWithUser(context.Background(), userID, value); err != nil {
				return "", err
			}
		case "status_text":
			if err := userClient.SetUserCustomStatusContextWithUser(context.Background(), userID, value, "", 0); err != nil {
				return "", err
			}
		case "status_emoji":
			if err := userClient.SetUserCustomStatusContextWithUser(context.Background(), userID, "", value, 0); err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("unsupported field %q; supported: real_name, status_text, status_emoji", name)
		}
		return okJSON(map[string]any{"updated": name, "value": value}), nil
	}))

	s.AddTool(mcp.NewTool("set_user_presence",
		mcp.WithDescription("Set the caller's presence. Requires SLACK_USER_TOKEN."),
		mcp.WithString("presence", mcp.Required(), mcp.Description("\"auto\" or \"away\".")),
	), wrap("set_user_presence", func(req mcp.CallToolRequest) (string, error) {
		userClient, err := c.User()
		if err != nil {
			return "", err
		}
		presence := strArg(req, "presence")
		if err := userClient.SetUserPresence(presence); err != nil {
			return "", err
		}
		return okJSON(map[string]any{"presence": presence}), nil
	}))
}

func matchesUser(u slack.User, query string, fields []string) bool {
	for _, f := range fields {
		var s string
		switch strings.TrimSpace(f) {
		case "name":
			s = u.Name
		case "real_name":
			s = u.RealName
		case "display_name":
			s = u.Profile.DisplayName
		case "email":
			s = u.Profile.Email
		default:
			continue
		}
		if strings.Contains(strings.ToLower(s), query) {
			return true
		}
	}
	return false
}
