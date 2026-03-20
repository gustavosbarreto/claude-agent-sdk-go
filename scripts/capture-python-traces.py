#!/usr/bin/env python3
"""Capture NDJSON traces from the Python SDK e2e tests.

Patches the ClaudeAgentOptions default cli_path to use the sniffer,
then runs a simple test that exercises the main SDK features.

Usage:
    CLAUDE_SNIFFER_DIR=/tmp/claude-traces/python python3 scripts/capture-python-traces.py
"""

import asyncio
import os
import sys

# Must have claude-agent-sdk installed
try:
    from claude_agent_sdk import (
        ClaudeAgentOptions,
        ClaudeSDKClient,
        query,
    )
except ImportError:
    print("Install: pip install claude-agent-sdk", file=sys.stderr)
    sys.exit(1)

SNIFFER = os.path.join(os.path.dirname(__file__), "claude-sniffer.sh")
TRACE_DIR = os.environ.get("CLAUDE_SNIFFER_DIR", "/tmp/claude-traces/python")
os.makedirs(TRACE_DIR, exist_ok=True)
os.environ["CLAUDE_SNIFFER_DIR"] = TRACE_DIR


async def trace_prompt():
    """Simple prompt — captures basic request/response."""
    print("=== trace: prompt ===")
    options = ClaudeAgentOptions(
        cli_path=SNIFFER,
        max_turns=1,
    )
    async for msg in query(prompt="What is 2+2? Just the number.", options=options):
        print(f"  {type(msg).__name__}")


async def trace_session():
    """Multi-turn session — captures init, multi-turn, control protocol."""
    print("=== trace: session ===")
    options = ClaudeAgentOptions(
        cli_path=SNIFFER,
        max_turns=3,
    )
    async with ClaudeSDKClient(options=options) as client:
        await client.query("Pick a number between 1 and 10. Just the number.")
        async for msg in client.receive_response():
            print(f"  turn1: {type(msg).__name__}")

        await client.query("Double it. Just the number.")
        async for msg in client.receive_response():
            print(f"  turn2: {type(msg).__name__}")


async def trace_hooks():
    """Hooks — captures hook registration and callback protocol."""
    print("=== trace: hooks ===")

    invocations = []

    async def pre_hook(input_data, tool_use_id, context):
        invocations.append(input_data.get("tool_name", ""))
        return {
            "hookSpecificOutput": {
                "hookEventName": "PreToolUse",
                "permissionDecision": "allow",
            },
        }

    options = ClaudeAgentOptions(
        cli_path=SNIFFER,
        allowed_tools=["Bash"],
        hooks={
            "PreToolUse": [
                {"matcher": "Bash", "hooks": [pre_hook]},
            ],
        },
    )
    async with ClaudeSDKClient(options=options) as client:
        await client.query("Run: echo 'trace test'")
        async for msg in client.receive_response():
            print(f"  {type(msg).__name__}")

    print(f"  hooks fired: {len(invocations)}")


async def trace_structured_output():
    """Structured output — captures json-schema flag and structured_output field."""
    print("=== trace: structured_output ===")
    options = ClaudeAgentOptions(
        cli_path=SNIFFER,
        output_format={
            "type": "json_schema",
            "schema": {
                "type": "object",
                "properties": {"answer": {"type": "number"}},
                "required": ["answer"],
            },
        },
    )
    async for msg in query(prompt="What is 7*8?", options=options):
        print(f"  {type(msg).__name__}")


async def main():
    print(f"Sniffer: {SNIFFER}")
    print(f"Traces:  {TRACE_DIR}")
    print()

    await trace_prompt()
    await trace_session()
    await trace_hooks()
    await trace_structured_output()

    print()
    traces = [f for f in os.listdir(TRACE_DIR) if f.endswith(".ndjson")]
    print(f"Captured {len(traces)} trace files in {TRACE_DIR}")


if __name__ == "__main__":
    asyncio.run(main())
