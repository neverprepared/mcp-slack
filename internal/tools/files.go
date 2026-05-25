package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/neverprepared/mcp-slack/internal/client"
	"github.com/slack-go/slack"
)

func RegisterFileTools(s *server.MCPServer, c *client.SlackClient) {
	s.AddTool(mcp.NewTool("upload_file",
		mcp.WithDescription("Upload a file to Slack."),
		mcp.WithString("local_path", mcp.Required(), mcp.Description("Absolute path to local file.")),
		mcp.WithString("channel", mcp.Description("Channel ID to share into.")),
		mcp.WithString("title", mcp.Description("File title.")),
		mcp.WithString("initial_comment", mcp.Description("Message text to post with the file.")),
		mcp.WithString("thread_ts", mcp.Description("Thread parent ts to upload into.")),
		mcp.WithString("filename", mcp.Description("Override filename (default: basename of local_path).")),
	), wrap("upload_file", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		localPath := strArg(req, "local_path")
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return "", fmt.Errorf("local file not found: %s", localPath)
		}
		filename := strDefault(req, "filename", filepath.Base(localPath))
		params := slack.UploadFileV2Parameters{
			File:     localPath,
			Filename: filename,
		}
		if v, ok := optStr(req, "channel"); ok {
			params.Channel = v
		}
		if v, ok := optStr(req, "title"); ok {
			params.Title = v
		}
		if v, ok := optStr(req, "initial_comment"); ok {
			params.InitialComment = v
		}
		if v, ok := optStr(req, "thread_ts"); ok {
			params.ThreadTimestamp = v
		}
		file, err := bot.UploadFileV2(params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"files": []any{file}}), nil
	}))

	s.AddTool(mcp.NewTool("list_files",
		mcp.WithDescription("List files in the workspace, optionally filtered."),
		mcp.WithString("user", mcp.Description("Filter to files uploaded by this user ID.")),
		mcp.WithString("channel", mcp.Description("Filter to files shared in this channel.")),
		mcp.WithNumber("ts_from", mcp.Description("Unix epoch lower bound on upload time.")),
		mcp.WithNumber("ts_to", mcp.Description("Unix epoch upper bound on upload time.")),
		mcp.WithString("types", mcp.Description("Comma-separated types (all, spaces, snippets, images, gdocs, zips, pdfs).")),
		mcp.WithNumber("count", mcp.Description("Results per page (default 100).")),
		mcp.WithNumber("page", mcp.Description("Page number, 1-indexed (default 1).")),
	), wrap("list_files", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		params := slack.GetFilesParameters{
			Count: intDefault(req, "count", 100),
			Page:  intDefault(req, "page", 1),
		}
		if v, ok := optStr(req, "user"); ok {
			params.User = v
		}
		if v, ok := optStr(req, "channel"); ok {
			params.Channel = v
		}
		if v, ok := optStr(req, "types"); ok {
			params.Types = v
		}
		if v := intDefault(req, "ts_from", 0); v != 0 {
			params.TimestampFrom = slack.JSONTime(v)
		}
		if v := intDefault(req, "ts_to", 0); v != 0 {
			params.TimestampTo = slack.JSONTime(v)
		}
		files, paging, err := bot.GetFiles(params)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"files": files, "paging": paging}), nil
	}))

	s.AddTool(mcp.NewTool("get_file_info",
		mcp.WithDescription("Get info about a file."),
		mcp.WithString("file", mcp.Required(), mcp.Description("File ID.")),
		mcp.WithNumber("count", mcp.Description("Comments per page (default 100).")),
		mcp.WithNumber("page", mcp.Description("Page number (default 1).")),
	), wrap("get_file_info", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		file, comments, paging, err := bot.GetFileInfo(
			strArg(req, "file"),
			intDefault(req, "count", 100),
			intDefault(req, "page", 1),
		)
		if err != nil {
			return "", err
		}
		return okJSON(map[string]any{"file": file, "comments": comments, "paging": paging}), nil
	}))

	s.AddTool(mcp.NewTool("delete_file",
		mcp.WithDescription("Delete a file by ID."),
		mcp.WithString("file", mcp.Required(), mcp.Description("File ID.")),
	), wrap("delete_file", func(req mcp.CallToolRequest) (string, error) {
		bot, err := c.Bot()
		if err != nil {
			return "", err
		}
		id := strArg(req, "file")
		if err := bot.DeleteFile(id); err != nil {
			return "", err
		}
		return okJSON(map[string]any{"deleted": id}), nil
	}))
}
