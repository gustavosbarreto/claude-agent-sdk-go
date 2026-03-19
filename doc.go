// Package claude provides a Go SDK for the Claude Code CLI.
//
// It communicates with the Claude Code CLI subprocess via NDJSON over stdin/stdout,
// supporting one-shot prompts, multi-turn sessions, hooks, MCP servers, and subagents.
//
// Basic usage:
//
//	result, err := claude.Prompt(ctx, "What is 2+2?")
//
// Multi-turn session:
//
//	session := claude.NewSession()
//	for msg := range session.Send(ctx, "Hello") {
//	    // handle messages
//	}
//	for msg := range session.Send(ctx, "Follow up") {
//	    // handle messages
//	}
//	session.Close()
package claude
