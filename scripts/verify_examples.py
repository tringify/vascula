#!/usr/bin/env python3
"""Compile and run every ```go run block in docs/*.md as an independent
consumer of the module, and compare its output with the ```output block that
follows it."""
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile


def blocks(text):
    """Fenced blocks as (lang, flags, body, adjacent-to-previous)."""
    out, lines, i, only_blank = [], text.split("\n"), 0, False
    while i < len(lines):
        stripped = lines[i].strip()
        if stripped.startswith("```"):
            info = stripped[3:].split()
            body = []
            i += 1
            while i < len(lines) and lines[i].strip() != "```":
                body.append(lines[i])
                i += 1
            out.append((info[0] if info else "", info[1:], "\n".join(body), only_blank))
            only_blank = True
        elif stripped:
            only_blank = False
        i += 1
    return out


def verify():
    root = Path(__file__).resolve().parents[1]
    env = dict(os.environ, GOWORK="off", GOPROXY="off", GOSUMDB="off")
    count = 0
    with tempfile.TemporaryDirectory(prefix="vascula-examples-") as temporary:
        work = Path(temporary)
        (work / "go.mod").write_text(
            "module example.com/vascula-docs-consumer\n\ngo 1.25.0\n"
            "require vascula.dev/vascula v0.0.0\n"
            "replace vascula.dev/vascula => " + json.dumps(str(root)) + "\n"
        )
        for page in sorted((root / "docs").glob("*.md")):
            found = blocks(page.read_text())
            for index, (lang, flags, body, _) in enumerate(found):
                if lang != "go" or "run" not in flags:
                    continue
                (work / "main.go").write_text(body + "\n")
                result = subprocess.run(["go", "run", "."], cwd=work, env=env, capture_output=True, text=True, timeout=120)
                where = f"{page.name} (example {index + 1})"
                if result.returncode:
                    raise SystemExit(f"Go example {where} failed:\n{result.stderr}")
                if index + 1 < len(found) and found[index + 1][0] == "output" and found[index + 1][3]:
                    want = found[index + 1][2].rstrip("\n")
                    got = result.stdout.rstrip("\n")
                    if got != want:
                        raise SystemExit(f"Go example {where} printed:\n{got}\nwant:\n{want}")
                count += 1
    if count < 10:
        raise SystemExit(f"only {count} runnable Go examples")
    print(f"Verified {count} runnable Go documentation examples")


if __name__ == "__main__":
    verify()
