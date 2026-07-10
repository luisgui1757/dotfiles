#!/usr/bin/env python3
"""Reject Nix/Home Manager options that would take ownership from chezmoi."""

from __future__ import annotations

import argparse
import dataclasses
import pathlib
import re
import sys


@dataclasses.dataclass(frozen=True)
class Token:
    value: str
    line: int


TOKEN_RE = re.compile(
    r"""
    (?P<space>\s+)
  | (?P<line_comment>\#[^\n]*)
  | (?P<block_comment>/\*.*?\*/)
  | (?P<string>"(?:\\.|[^"\\])*")
  | (?P<identifier>[A-Za-z_][A-Za-z0-9_'-]*)
  | (?P<punct>[.={};:\[\](),])
  | (?P<other>.)
    """,
    re.DOTALL | re.VERBOSE,
)


def tokenize(source: str) -> list[Token]:
    tokens: list[Token] = []
    line = 1
    for match in TOKEN_RE.finditer(source):
        kind = match.lastgroup
        value = match.group(0)
        if kind not in {"space", "line_comment", "block_comment"}:
            tokens.append(Token(value=value, line=line))
        line += value.count("\n")
    return tokens


def prohibited_option(parts: tuple[str, ...]) -> str | None:
    for index, part in enumerate(parts):
        tail = parts[index:]
        if len(tail) >= 2 and tail[:2] in {
            ("home", "file"),
            ("home", "activation"),
        }:
            return ".".join(tail[:2])
        if len(tail) >= 2 and tail[0] == "xdg" and tail[1] in {
            "configFile",
            "dataFile",
            "desktopEntries",
        }:
            return ".".join(tail[:2])
        if len(tail) >= 2 and tail[0] == "programs" and tail[1] != "home-manager":
            return ".".join(tail[:2])
    return None


def attr_part(token: Token) -> str | None:
    if re.fullmatch(r"[A-Za-z_][A-Za-z0-9_'-]*", token.value):
        return token.value
    if token.value.startswith('"'):
        return "<quoted>"
    return None


def matching_close(tokens: list[Token], start: int, end: int) -> int:
    pairs = {"{": "}", "[": "]", "(": ")"}
    expected = pairs[tokens[start].value]
    depth = 1
    for index in range(start + 1, end):
        value = tokens[index].value
        if value == tokens[start].value:
            depth += 1
        elif value == expected:
            depth -= 1
            if depth == 0:
                return index
    return end


def assignment_end(tokens: list[Token], start: int, end: int) -> int:
    stack: list[str] = []
    pairs = {"{": "}", "[": "]", "(": ")"}
    for index in range(start, end):
        value = tokens[index].value
        if value in pairs:
            stack.append(pairs[value])
        elif stack and value == stack[-1]:
            stack.pop()
        elif value == ";" and not stack:
            return index
    return end


def scan_tokens(
    tokens: list[Token],
    path: pathlib.Path,
    start: int = 0,
    end: int | None = None,
    prefix: tuple[str, ...] = (),
) -> list[str]:
    if end is None:
        end = len(tokens)
    findings: list[str] = []
    index = start
    while index < end:
        token = tokens[index]
        if token.value == "{":
            close = matching_close(tokens, index, end)
            findings.extend(scan_tokens(tokens, path, index + 1, close, prefix))
            index = close + 1
            continue

        if not re.fullmatch(r"[A-Za-z_][A-Za-z0-9_'-]*", token.value):
            index += 1
            continue

        parts = [token.value]
        cursor = index + 1
        while cursor + 1 < end and tokens[cursor].value == "." and attr_part(tokens[cursor + 1]):
            parts.append(attr_part(tokens[cursor + 1]) or "<invalid>")
            cursor += 2
        if cursor >= end or tokens[cursor].value != "=":
            index += 1
            continue

        full_path = prefix + tuple(parts)
        violation = prohibited_option(full_path)
        if violation:
            findings.append(f"{path}:{token.line}: prohibited dotfile ownership option {violation}")

        value_start = cursor + 1
        value_end = assignment_end(tokens, value_start, end)
        # Propagate the assigned option prefix through wrappers such as mkMerge,
        # mkIf, lists, and nested attrsets. That closes the common `home = {
        # file = ...; };` and `home = mkMerge [{ file = ...; }];` bypasses.
        findings.extend(scan_tokens(tokens, path, value_start, value_end, full_path))
        index = value_end + 1
    return findings


def scan_source(source: str, path: pathlib.Path) -> list[str]:
    return scan_tokens(tokenize(source), path)


def self_test() -> None:
    allowed = """
    {
      home.packages = [];
      programs.home-manager.enable = true;
      system.defaults.dock.autohide = true;
      environment.etc."example".text = "programs.zsh.enable = true";
      launchd.daemons.example = {};
      # home.file."ignored".text = "comment";
    }
    """
    if scan_source(allowed, pathlib.Path("allowed.nix")):
        raise AssertionError("legitimate system-policy options were rejected")

    cases = {
        "direct.nix": "{ home.file.\"x\".text = \"x\"; }",
        "nested.nix": "{ home = { file = { x.text = \"x\"; }; }; }",
        "wrapped.nix": "{ home = lib.mkMerge [{ activation = { x = \"bad\"; }; }]; }",
        "imported-child.nix": "{ programs = { zsh.enable = true; }; }",
        "xdg.nix": "{ xdg = { configFile = { x.text = \"x\"; }; }; }",
    }
    for name, source in cases.items():
        if not scan_source(source, pathlib.Path(name)):
            raise AssertionError(f"{name} bypassed the ownership scanner")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--self-test", action="store_true")
    parser.add_argument("files", nargs="*", type=pathlib.Path)
    args = parser.parse_args()

    if args.self_test:
        self_test()
        print("OK: Nix ownership scanner rejects nested and imported bypass shapes")
        return 0

    findings: list[str] = []
    for path in args.files:
        findings.extend(scan_source(path.read_text(encoding="utf-8"), path))
    if findings:
        print("\n".join(findings))
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
