#!/usr/bin/env python3
"""Extract conformance test fixtures from the official Python SDK.

Usage:
    python3 scripts/extract-fixtures.py /path/to/claude-agent-sdk-python > testdata/conformance.json

Reads tests/test_message_parser.py from the Python SDK and extracts all
test input dicts into a JSON fixture file that our Go conformance tests consume.
"""

import ast
import json
import sys
from pathlib import Path


def extract_test_cases(python_sdk_path: str) -> list[dict]:
    """Parse the Python test file's AST and extract test case dicts."""
    test_file = Path(python_sdk_path) / "tests" / "test_message_parser.py"
    if not test_file.exists():
        print(f"Error: {test_file} not found", file=sys.stderr)
        sys.exit(1)

    source = test_file.read_text()
    tree = ast.parse(source)

    cases = []

    for node in ast.walk(tree):
        if not isinstance(node, ast.FunctionDef):
            continue
        if not node.name.startswith("test_parse_"):
            continue

        # Find dict assignments named 'data'
        for stmt in ast.walk(node):
            if not isinstance(stmt, ast.Assign):
                continue
            for target in stmt.targets:
                if isinstance(target, ast.Name) and target.id == "data":
                    try:
                        # Evaluate the dict literal safely
                        data = ast.literal_eval(stmt.value)
                        if isinstance(data, dict) and "type" in data:
                            case = {
                                "name": node.name.removeprefix("test_parse_"),
                                "input": data,
                                "expect_type": data["type"],
                            }
                            if "subtype" in data:
                                case["expect_subtype"] = data["subtype"]
                            cases.append(case)
                    except (ValueError, TypeError):
                        # Skip non-literal dicts
                        pass

    return cases


def main():
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} /path/to/claude-agent-sdk-python", file=sys.stderr)
        sys.exit(1)

    cases = extract_test_cases(sys.argv[1])

    output = {
        "_comment": "Auto-extracted from anthropics/claude-agent-sdk-python tests/test_message_parser.py",
        "_source": "scripts/extract-fixtures.py",
        "cases": cases,
    }

    print(json.dumps(output, indent=2))
    print(f"Extracted {len(cases)} test cases", file=sys.stderr)


if __name__ == "__main__":
    main()
