package mcp

import (
	"context"
	"log/slog"
	"os"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/leporel/huetension/internal/httputil"
)

// resolveLogger returns the *slog.Logger to use for this server. When the
// caller pre-set cfg.Logger (tests, embedding hosts), that instance is
// returned verbatim. Otherwise httputil.NewLogger builds one writing to
// stderr in cfg.LogFormat at cfg.LogLevel.
//
// Stderr is deliberate: the stdio transport speaks JSON-RPC on stdout, so
// any log byte on stdout would corrupt the wire. Anything we own writes
// to stderr.
func resolveLogger(cfg Config) (*slog.Logger, error) {
	if cfg.Logger != nil {
		return cfg.Logger, nil
	}
	return httputil.NewLogger(os.Stderr, cfg.LogFormat, cfg.LogLevel)
}

// loggingMiddleware logs every receiving method dispatch as a single info
// entry: method, duration, error status, plus a method-specific subject
// (tool name, resource URI, prompt name) when one is naturally available.
//
// Tool *arguments* are intentionally not logged — they may contain user
// data (image paths, color strings tagged with internal codenames, etc.).
// At debug level we log argument types only, never their values. If a
// future caller needs full argument logging, they should set their own
// cfg.Logger with redaction in place.
func loggingMiddleware(logger *slog.Logger) sdk.Middleware {
	return func(next sdk.MethodHandler) sdk.MethodHandler {
		return func(ctx context.Context, method string, req sdk.Request) (sdk.Result, error) {
			start := time.Now()
			result, err := next(ctx, method, req)
			attrs := []any{
				slog.String("method", method),
				slog.Duration("duration", time.Since(start)),
			}
			if sess := req.GetSession(); sess != nil {
				if id := sess.ID(); id != "" {
					attrs = append(attrs, slog.String("session_id", id))
				}
			}
			if subject, ok := methodSubject(method, req); ok {
				attrs = append(attrs, slog.String("subject", subject))
			}
			if err != nil {
				attrs = append(attrs, slog.String("error", err.Error()))
				logger.LogAttrs(ctx, slog.LevelError, "mcp.call", toAttrs(attrs)...)
				return result, err
			}
			logger.LogAttrs(ctx, slog.LevelInfo, "mcp.call", toAttrs(attrs)...)
			return result, nil
		}
	}
}

// methodSubject pulls a stable, safe-to-log identifier out of req.Params
// for the methods where one exists. Returns ("", false) for methods that
// don't carry a meaningful subject (initialise, list, ping, …).
func methodSubject(method string, req sdk.Request) (string, bool) {
	params := req.GetParams()
	if params == nil {
		return "", false
	}
	switch method {
	case "tools/call":
		if p, ok := params.(*sdk.CallToolParamsRaw); ok && p != nil {
			return p.Name, p.Name != ""
		}
		if p, ok := params.(*sdk.CallToolParams); ok && p != nil {
			return p.Name, p.Name != ""
		}
	case "resources/read":
		if p, ok := params.(*sdk.ReadResourceParams); ok && p != nil {
			return p.URI, p.URI != ""
		}
	case "prompts/get":
		if p, ok := params.(*sdk.GetPromptParams); ok && p != nil {
			return p.Name, p.Name != ""
		}
	}
	return "", false
}

// toAttrs converts the loose []any slog argument list into []slog.Attr.
// Required because LogAttrs accepts []slog.Attr, not the variadic []any
// slog.Logger.Info uses. Centralising the conversion keeps the call sites
// readable without a wrapper helper per call.
func toAttrs(in []any) []slog.Attr {
	out := make([]slog.Attr, 0, len(in))
	for _, v := range in {
		if a, ok := v.(slog.Attr); ok {
			out = append(out, a)
		}
	}
	return out
}

