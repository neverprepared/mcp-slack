// Search APIs require a user token (xoxp-). Slack does not allow bots to call search.*.
package tools

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neverprepared/mcp-slack/internal/client"
	"github.com/slack-go/slack"
)

func RegisterSearchTools(s *server.MCPServer, c *client.SlackClient) {
	s.AddTool(mcp.NewTool("search_messages",
		mcp.WithDescription("Search messages across the workspace. Requires SLACK_USER_TOKEN. Supports Slack modifiers (in:#channel, from:@user, before:, after:, has:, etc.)."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query.")),
		mcp.WithString("sort", mcp.Description("\"score\" (relevance, default) or \"timestamp\".")),
		mcp.WithString("sort_dir", mcp.Description("\"desc\" (default) or \"asc\".")),
		mcp.WithNumber("count", mcp.Description("Results per page (default 20, max 100).")),
		mcp.WithNumber("page", mcp.Description("Page number, 1-indexed.")),
		mcp.WithBoolean("highlight", mcp.Description("Mark matched terms in results.")),
	), wrap("search_messages", func(req mcp.CallToolRequest) (string, error) {
		userClient, err := c.User()
		if err != nil {
			return "", err
		}
		params := slack.SearchParameters{
			Sort:          strDefault(req, "sort", "score"),
			SortDirection: strDefault(req, "sort_dir", "desc"),
			Count:         intDefault(req, "count", 20),
			Page:          intDefault(req, "page", 1),
			Highlight:     boolDefault(req, "highlight", false),
		}
		msgs, err := userClient.SearchMessages(strArg(req, "query"), params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"messages": msgs}), nil
	}))

	s.AddTool(mcp.NewTool("search_files",
		mcp.WithDescription("Search files across the workspace. Requires SLACK_USER_TOKEN."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query.")),
		mcp.WithString("sort", mcp.Description("\"score\" or \"timestamp\".")),
		mcp.WithString("sort_dir", mcp.Description("\"desc\" or \"asc\".")),
		mcp.WithNumber("count", mcp.Description("Results per page (max 100).")),
		mcp.WithNumber("page", mcp.Description("Page number.")),
		mcp.WithBoolean("highlight", mcp.Description("Mark matched terms.")),
	), wrap("search_files", func(req mcp.CallToolRequest) (string, error) {
		userClient, err := c.User()
		if err != nil {
			return "", err
		}
		params := slack.SearchParameters{
			Sort:          strDefault(req, "sort", "score"),
			SortDirection: strDefault(req, "sort_dir", "desc"),
			Count:         intDefault(req, "count", 20),
			Page:          intDefault(req, "page", 1),
			Highlight:     boolDefault(req, "highlight", false),
		}
		files, err := userClient.SearchFiles(strArg(req, "query"), params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"files": files}), nil
	}))

	s.AddTool(mcp.NewTool("search_all",
		mcp.WithDescription("Search both messages and files in one call. Requires SLACK_USER_TOKEN."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query.")),
		mcp.WithString("sort", mcp.Description("\"score\" or \"timestamp\".")),
		mcp.WithString("sort_dir", mcp.Description("\"desc\" or \"asc\".")),
		mcp.WithNumber("count", mcp.Description("Results per page.")),
		mcp.WithNumber("page", mcp.Description("Page number.")),
	), wrap("search_all", func(req mcp.CallToolRequest) (string, error) {
		userClient, err := c.User()
		if err != nil {
			return "", err
		}
		params := slack.SearchParameters{
			Sort:          strDefault(req, "sort", "score"),
			SortDirection: strDefault(req, "sort_dir", "desc"),
			Count:         intDefault(req, "count", 20),
			Page:          intDefault(req, "page", 1),
		}
		msgs, files, err := userClient.Search(strArg(req, "query"), params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"messages": msgs, "files": files}), nil
	}))
}
