package tools

import (
	"context"
	"encoding/json"
	"log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func okJSON(data map[string]any) string {
	data["status"] = "success"
	b, _ := json.Marshal(data)
	return string(b)
}

func errJSON(msg string) string {
	b, _ := json.Marshal(map[string]any{"status": "error", "error": msg})
	return string(b)
}

// wrap converts a typed tool handler into a server.ToolHandlerFunc.
// All errors (Slack API, validation, missing tokens) are returned as JSON
// rather than MCP-level errors, matching the Python @slack_tool decorator.
func wrap(name string, fn func(mcp.CallToolRequest) (string, error)) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		result, err := fn(req)
		if err != nil {
			log.Printf("%s error: %v", name, err)
			return mcp.NewToolResultText(errJSON(err.Error())), nil
		}
		return mcp.NewToolResultText(result), nil
	}
}

// args returns the request arguments as map[string]any.
func args(req mcp.CallToolRequest) map[string]any {
	a := req.GetArguments()
	if a == nil {
		return map[string]any{}
	}
	return a
}

// Arg helpers — JSON numbers come through as float64 over the wire.

func strArg(req mcp.CallToolRequest, key string) string {
	v, _ := args(req)[key].(string)
	return v
}

func strDefault(req mcp.CallToolRequest, key, def string) string {
	v, ok := args(req)[key]
	if !ok || v == nil {
		return def
	}
	s, _ := v.(string)
	if s == "" {
		return def
	}
	return s
}

func boolDefault(req mcp.CallToolRequest, key string, def bool) bool {
	v, ok := args(req)[key]
	if !ok || v == nil {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}

func intDefault(req mcp.CallToolRequest, key string, def int) int {
	v, ok := args(req)[key]
	if !ok || v == nil {
		return def
	}
	f, ok := v.(float64)
	if !ok {
		return def
	}
	return int(f)
}

// optStr returns the string value and true if the key is present and non-empty.
func optStr(req mcp.CallToolRequest, key string) (string, bool) {
	v, ok := args(req)[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return s, ok && s != ""
}
