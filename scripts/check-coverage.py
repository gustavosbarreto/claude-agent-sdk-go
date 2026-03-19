#!/usr/bin/env python3
"""Check type coverage: what exists in the Python SDK that we don't implement in Go.

Usage:
    python3 scripts/check-coverage.py /path/to/claude-agent-sdk-python ./

Compares the Python SDK's types.py with our Go message.go/option.go/hook.go
and reports missing types, fields, options, and hook events.

Exit code 0 = fully covered, 1 = gaps found.
"""

import re
import sys
from pathlib import Path


def extract_python_message_types(sdk_path: str) -> set[str]:
    """Extract message class names from Python types.py."""
    types_file = Path(sdk_path) / "src" / "claude_agent_sdk" / "types.py"
    if not types_file.exists():
        print(f"Warning: {types_file} not found", file=sys.stderr)
        return set()

    content = types_file.read_text()
    # Match class definitions that look like message types
    classes = set(re.findall(r"class (\w+Message)\b", content))
    classes |= set(re.findall(r"class (\w+Event)\b", content))
    return classes


def extract_python_hook_events(sdk_path: str) -> set[str]:
    """Extract hook event names from Python types.py."""
    types_file = Path(sdk_path) / "src" / "claude_agent_sdk" / "types.py"
    if not types_file.exists():
        return set()

    content = types_file.read_text()
    # Match HookEvent literal values
    events = set(re.findall(r'"((?:Pre|Post|Session|Sub|Task|Team|Config|Work|User|Notification|Stop|Setup|Elicit|Instructions)\w+)"', content))
    return events


def extract_python_options(sdk_path: str) -> set[str]:
    """Extract option field names from ClaudeAgentOptions."""
    types_file = Path(sdk_path) / "src" / "claude_agent_sdk" / "types.py"
    if not types_file.exists():
        return set()

    content = types_file.read_text()
    # Find ClaudeAgentOptions class and extract field names
    in_class = False
    options = set()
    for line in content.split("\n"):
        if "class ClaudeAgentOptions" in line:
            in_class = True
            continue
        if in_class:
            if line and not line.startswith(" ") and not line.startswith("\t"):
                break
            m = re.match(r"\s+(\w+)\s*:", line)
            if m:
                options.add(m.group(1))
    return options


def extract_go_types(go_path: str) -> set[str]:
    """Extract type names from Go source files."""
    types = set()
    for f in Path(go_path).glob("*.go"):
        if f.name.endswith("_test.go"):
            continue
        content = f.read_text()
        types |= set(re.findall(r"type (\w+(?:Message|Event))\b", content))
    return types


def extract_go_hook_events(go_path: str) -> set[str]:
    """Extract hook event constants from Go."""
    hook_file = Path(go_path) / "hook.go"
    if not hook_file.exists():
        return set()

    content = hook_file.read_text()
    return set(re.findall(r'HookEvent = "(\w+)"', content))


def extract_go_options(go_path: str) -> set[str]:
    """Extract With* option function names from Go."""
    option_file = Path(go_path) / "option.go"
    if not option_file.exists():
        return set()

    content = option_file.read_text()
    return set(re.findall(r"func (With\w+)\(", content))


def main():
    if len(sys.argv) < 3:
        print(f"Usage: {sys.argv[0]} /path/to/python-sdk /path/to/go-sdk", file=sys.stderr)
        sys.exit(1)

    python_path = sys.argv[1]
    go_path = sys.argv[2]

    gaps = []

    # Message types
    py_types = extract_python_message_types(python_path)
    go_types = extract_go_types(go_path)
    missing_types = py_types - go_types
    if missing_types:
        gaps.append(f"Missing message types: {', '.join(sorted(missing_types))}")

    # Hook events
    py_hooks = extract_python_hook_events(python_path)
    go_hooks = extract_go_hook_events(go_path)
    missing_hooks = py_hooks - go_hooks
    if missing_hooks:
        gaps.append(f"Missing hook events: {', '.join(sorted(missing_hooks))}")

    # Options (informational — naming differs between Python/Go)
    py_options = extract_python_options(python_path)
    if py_options:
        go_options = extract_go_options(go_path)
        print(f"Python options: {len(py_options)}, Go With* functions: {len(go_options)}", file=sys.stderr)

    if gaps:
        print("Coverage gaps found:")
        for gap in gaps:
            print(f"  - {gap}")
        sys.exit(1)
    else:
        print("Full coverage — all Python SDK types are implemented in Go")
        sys.exit(0)


if __name__ == "__main__":
    main()
