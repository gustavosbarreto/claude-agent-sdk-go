#!/usr/bin/env python3
"""Compare NDJSON traces between Python SDK and Go SDK.

Usage:
    python3 scripts/compare-traces.py /tmp/claude-traces/python /tmp/claude-traces/go

Compares:
- CLI arguments passed
- Control protocol messages (initialize, hook_callback, mcp_message, can_use_tool)
- Message types and ordering
- Fields present in each message
"""

import json
import os
import sys
from pathlib import Path


def load_traces(trace_dir: str) -> list[dict]:
    """Load all trace sessions from a directory."""
    sessions = []
    trace_path = Path(trace_dir)

    # Group files by prefix (tag_timestamp or just timestamp).
    # Extract the full prefix before _args/_stdin/_stdout/_stderr.
    prefixes = set()
    for f in trace_path.iterdir():
        name = f.name
        for suffix in ("_args.txt", "_stdin.ndjson", "_stdout.ndjson", "_stderr.txt"):
            if name.endswith(suffix):
                prefix = name[: -len(suffix)]
                prefixes.add(prefix)

    for prefix in sorted(prefixes):
        # Extract tag if present (tag_timestamp format).
        parts = prefix.rsplit("_", 1)
        tag = parts[0] if len(parts) > 1 and not parts[0].isdigit() else ""

        session = {"prefix": prefix, "tag": tag}

        args_file = trace_path / f"{prefix}_args.txt"
        if args_file.exists():
            session["args"] = args_file.read_text().strip()

        stdin_file = trace_path / f"{prefix}_stdin.ndjson"
        if stdin_file.exists():
            session["stdin"] = parse_ndjson(stdin_file.read_text())

        stdout_file = trace_path / f"{prefix}_stdout.ndjson"
        if stdout_file.exists():
            session["stdout"] = parse_ndjson(stdout_file.read_text())

        stderr_file = trace_path / f"{prefix}_stderr.txt"
        if stderr_file.exists():
            session["stderr"] = stderr_file.read_text().strip()

        sessions.append(session)

    return sessions


def parse_ndjson(text: str) -> list[dict]:
    """Parse NDJSON text into list of dicts."""
    messages = []
    for line in text.strip().split("\n"):
        line = line.strip()
        if not line:
            continue
        try:
            messages.append(json.loads(line))
        except json.JSONDecodeError:
            messages.append({"_raw": line})
    return messages


def extract_message_types(messages: list[dict]) -> list[str]:
    """Extract message type sequence."""
    types = []
    for msg in messages:
        t = msg.get("type", "unknown")
        subtype = msg.get("subtype", "")
        if subtype:
            types.append(f"{t}/{subtype}")
        else:
            types.append(t)
    return types


def extract_cli_flags(args: str) -> dict:
    """Parse CLI args into a dict of flag -> value."""
    flags = {}
    parts = args.split()
    i = 0
    while i < len(parts):
        if parts[i].startswith("--"):
            flag = parts[i]
            if i + 1 < len(parts) and not parts[i + 1].startswith("--"):
                flags[flag] = parts[i + 1]
                i += 2
            else:
                flags[flag] = True
                i += 1
        else:
            i += 1
    return flags


def compare_sessions(py_sessions: list[dict], go_sessions: list[dict]):
    """Compare Python and Go trace sessions."""
    # Filter out version checks (args ending with '-v' or just a version command).
    py_real = [s for s in py_sessions if not s.get("args", "").rstrip().endswith("-v")]
    go_real = [s for s in go_sessions if not s.get("args", "").rstrip().endswith("-v")]

    print(f"Python sessions: {len(py_sessions)} total, {len(py_real)} real")
    print(f"Go sessions:     {len(go_sessions)} total, {len(go_real)} real")
    print()

    n = min(len(py_real), len(go_real))

    for i in range(n):
        py = py_real[i]
        go = go_real[i]

        label = py.get("tag") or go.get("tag") or f"session {i + 1}"
        print(f"=== {label} ===")

        # Compare CLI args
        py_flags = extract_cli_flags(py.get("args", ""))
        go_flags = extract_cli_flags(go.get("args", ""))

        py_only_flags = set(py_flags) - set(go_flags)
        go_only_flags = set(go_flags) - set(py_flags)
        diff_flags = {
            k for k in set(py_flags) & set(go_flags) if py_flags[k] != go_flags[k]
        }

        if py_only_flags or go_only_flags or diff_flags:
            print("  CLI args diff:")
            for f in sorted(py_only_flags):
                print(f"    Python only: {f} = {py_flags[f]}")
            for f in sorted(go_only_flags):
                print(f"    Go only:     {f} = {go_flags[f]}")
            for f in sorted(diff_flags):
                print(f"    Different:   {f}: Python={py_flags[f]} Go={go_flags[f]}")
        else:
            print("  CLI args: identical")

        # Compare stdin (SDK → CLI) message types
        py_stdin_types = extract_message_types(py.get("stdin", []))
        go_stdin_types = extract_message_types(go.get("stdin", []))

        if py_stdin_types != go_stdin_types:
            print(f"  stdin types diff:")
            print(f"    Python: {py_stdin_types}")
            print(f"    Go:     {go_stdin_types}")
        else:
            print(f"  stdin types: identical ({len(py_stdin_types)} messages)")

        # Compare stdout (CLI → SDK) message types
        py_stdout_types = extract_message_types(py.get("stdout", []))
        go_stdout_types = extract_message_types(go.get("stdout", []))

        if py_stdout_types != go_stdout_types:
            print(f"  stdout types diff:")
            print(f"    Python: {py_stdout_types}")
            print(f"    Go:     {go_stdout_types}")
        else:
            print(f"  stdout types: identical ({len(py_stdout_types)} messages)")

        # Compare control protocol messages in stdin
        py_control = [m for m in py.get("stdin", []) if m.get("type") in ("control_request", "control_response")]
        go_control = [m for m in go.get("stdin", []) if m.get("type") in ("control_request", "control_response")]

        py_control_subtypes = [
            m.get("request", {}).get("subtype", m.get("response", {}).get("subtype", "?"))
            for m in py_control
        ]
        go_control_subtypes = [
            m.get("request", {}).get("subtype", m.get("response", {}).get("subtype", "?"))
            for m in go_control
        ]

        if py_control_subtypes != go_control_subtypes:
            print(f"  control protocol diff:")
            print(f"    Python: {py_control_subtypes}")
            print(f"    Go:     {go_control_subtypes}")
        elif py_control_subtypes:
            print(f"  control protocol: identical ({py_control_subtypes})")

        print()


def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} <python-trace-dir> <go-trace-dir>")
        sys.exit(1)

    py_dir = sys.argv[1]
    go_dir = sys.argv[2]

    if not os.path.isdir(py_dir):
        print(f"Error: {py_dir} not found")
        sys.exit(1)
    if not os.path.isdir(go_dir):
        print(f"Error: {go_dir} not found")
        sys.exit(1)

    py_sessions = load_traces(py_dir)
    go_sessions = load_traces(go_dir)

    compare_sessions(py_sessions, go_sessions)


if __name__ == "__main__":
    main()
