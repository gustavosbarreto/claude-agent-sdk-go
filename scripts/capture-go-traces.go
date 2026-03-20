//go:build ignore

// Capture NDJSON traces from the Go SDK.
// Runs the same scenarios as capture-python-traces.py for comparison.
//
// Usage:
//
//	CLAUDE_SNIFFER_DIR=/tmp/claude-traces/go go run scripts/capture-go-traces.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	claude "github.com/shellhub-io/claude-agent-sdk-go"
)

const sniffer = "./scripts/claude-sniffer.sh"

func main() {
	traceDir := os.Getenv("CLAUDE_SNIFFER_DIR")
	if traceDir == "" {
		traceDir = "/tmp/claude-traces/go"
	}
	os.MkdirAll(traceDir, 0o755)
	os.Setenv("CLAUDE_SNIFFER_DIR", traceDir)

	fmt.Printf("Sniffer: %s\n", sniffer)
	fmt.Printf("Traces:  %s\n\n", traceDir)

	ctx := context.Background()

	tracePrompt(ctx)
	traceSession(ctx)
	traceHooks(ctx)
	traceStructuredOutput(ctx)

	entries, _ := os.ReadDir(traceDir)
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	fmt.Printf("\nCaptured %d trace files in %s\n", count, traceDir)
}

func tracePrompt(ctx context.Context) {
	fmt.Println("=== trace: prompt ===")
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, err := claude.Prompt(ctx, "What is 2+2? Just the number.",
		claude.WithCLIPath(sniffer),
		claude.WithMaxTurns(1),
		claude.WithNoPersistSession(),
	)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}
	fmt.Printf("  result: %s\n", result.Result)
}

func traceSession(ctx context.Context) {
	fmt.Println("=== trace: session ===")
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	session, err := claude.NewSession(ctx,
		claude.WithCLIPath(sniffer),
		claude.WithMaxTurns(3),
		claude.WithNoPersistSession(),
	)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}
	defer session.Close()

	for msg, err := range session.Send(ctx, "Pick a number between 1 and 10. Just the number.") {
		if err != nil {
			fmt.Printf("  turn1 error: %v\n", err)
			break
		}
		fmt.Printf("  turn1: %T\n", msg)
	}

	for msg, err := range session.Send(ctx, "Double it. Just the number.") {
		if err != nil {
			fmt.Printf("  turn2 error: %v\n", err)
			break
		}
		fmt.Printf("  turn2: %T\n", msg)
	}
}

func traceHooks(ctx context.Context) {
	fmt.Println("=== trace: hooks ===")
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	invocations := 0
	matcher := "Bash"

	session, err := claude.NewSession(ctx,
		claude.WithCLIPath(sniffer),
		claude.WithPermissionMode(claude.PermissionAcceptEdits),
		claude.WithAllowedTools("Bash"),
		claude.WithNoPersistSession(),
		claude.WithHook(claude.HookPreToolUse, claude.HookCallbackMatcher{
			Matcher: &matcher,
			Hooks: []claude.HookCallback{
				func(ctx context.Context, input claude.HookInput) (claude.HookOutput, error) {
					invocations++
					return claude.HookOutput{
						Decision: "allow",
					}, nil
				},
			},
		}),
	)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}
	defer session.Close()

	for msg, err := range session.Send(ctx, "Run: echo 'trace test'") {
		if err != nil {
			fmt.Printf("  error: %v\n", err)
			break
		}
		fmt.Printf("  %T\n", msg)
	}
	fmt.Printf("  hooks fired: %d\n", invocations)
}

func traceStructuredOutput(ctx context.Context) {
	fmt.Println("=== trace: structured_output ===")
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	result, err := claude.Prompt(ctx, "What is 7*8?",
		claude.WithCLIPath(sniffer),
		claude.WithOutputFormat(claude.OutputFormat{
			Type: "json_schema",
			Schema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"answer": map[string]any{"type": "number"}},
				"required":   []string{"answer"},
			},
		}),
		claude.WithNoPersistSession(),
	)
	if err != nil {
		fmt.Printf("  error: %v\n", err)
		return
	}
	fmt.Printf("  structured_output: %s\n", string(result.StructuredOutput))
}
