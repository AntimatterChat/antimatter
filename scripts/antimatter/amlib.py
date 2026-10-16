"""Helpers shared by the Antimatter check and report scripts (standard library only)."""

import fnmatch
import json
import os
import re
import subprocess

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))

# Paths that never hold code shipped to users: tests, fixtures, mocks, generated test data, e2e suites.
TEST_PATH_RE = re.compile(
    r"(_test\.go$|\.test\.[jt]sx?$|\.spec\.[jt]sx?$|/__snapshots__/|/__mocks__/|/tests?/|/testlib/|/testutils?/"
    r"|/mocks/|/storetest/|/searchtest/|/testdata/|test_helper|test_fixtures|apitestlib|/manualtesting/"
    r"|/e2e-tests/|^e2e-tests/|/fixtures?/)"
)

SOURCE_EXT_RE = re.compile(r"\.(go|ts|tsx|js|jsx|mjs|cjs)$")


def tracked_files(root=ROOT, pathspecs=()):
    """Lists the files tracked by git (or every file when git isn't available), relative to root."""
    try:
        out = subprocess.run(
            ["git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--", *pathspecs],
            check=True, capture_output=True,
        ).stdout.decode("utf-8", "replace")
        return [p for p in out.split("\0") if p]
    except (OSError, subprocess.CalledProcessError):
        files = []
        bases = pathspecs or ["."]
        for base in bases:
            for dirpath, dirnames, filenames in os.walk(os.path.join(root, base)):
                dirnames[:] = [d for d in dirnames if d not in (".git", "node_modules", "dist", "lib")]
                for name in filenames:
                    files.append(os.path.relpath(os.path.join(dirpath, name), root))
        return files


def is_test_path(path):
    return bool(TEST_PATH_RE.search("/" + path))


def is_comment_line(line):
    s = line.lstrip()
    return s.startswith(("//", "/*", "* ", "*/")) or s == "*" or s.startswith("#")


def load_catalog(path):
    """Loads an i18n catalog: the server's list of {id, translation} or the web apps' flat map."""
    with open(path, encoding="utf-8") as f:
        data = json.load(f)
    if isinstance(data, list):
        return {e["id"]: e.get("translation") for e in data}
    return data


def read_lines(path):
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            return f.read().splitlines()
    except (FileNotFoundError, IsADirectoryError):
        return []


def glob_match(path, pattern):
    """fnmatch where '**/' may also match nothing, so 'a/**/b' matches 'a/b'."""
    if fnmatch.fnmatchcase(path, pattern):
        return True
    return "**/" in pattern and fnmatch.fnmatchcase(path, pattern.replace("**/", ""))


def md_escape(text):
    return text.replace("|", "\\|").replace("\n", " ")


def write_text(path, text):
    os.makedirs(os.path.dirname(os.path.abspath(path)), exist_ok=True)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


def append_summary(markdown):
    """Appends to the GitHub job summary when running in Actions."""
    summary = os.environ.get("GITHUB_STEP_SUMMARY")
    if summary:
        with open(summary, "a", encoding="utf-8") as f:
            f.write(markdown.rstrip() + "\n\n")
