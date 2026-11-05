#!/usr/bin/env python3
"""Tests for vuln_report.py (standard library only):

    python3 scripts/antimatter/vuln_report_test.py
"""

import contextlib
import io
import json
import os
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import amlib  # noqa: E402
import vuln_report  # noqa: E402

NO_FIX = "GO-2026-5932"  # called, no fixed version: may be allowlisted
FIXED = "GO-2026-6860"  # called, fixed in v0.62.0
IMPORTED = "GO-2026-6866"  # imported only


def govulncheck_stream(*findings):
    """A govulncheck -format json stream: one osv message per advisory, then the findings."""
    msgs = [{"config": {"protocol_version": "v1.0.0"}}]
    for vid, fixed, function in findings:
        msgs.append({"osv": {"id": vid, "summary": f"summary of {vid}", "aliases": []}})
    for vid, fixed, function in findings:
        frame = {"module": "golang.org/x/net", "version": "v0.60.0", "package": "golang.org/x/net/html"}
        if function:
            frame["function"] = function
        msgs.append({"finding": {"osv": vid, "fixed_version": fixed, "trace": [frame]}})
    return "\n".join(json.dumps(m, indent=2) for m in msgs)


OSV = {"results": [{
    "source": {"path": os.path.join(amlib.ROOT, "server", "go.mod")},
    "packages": [{
        "package": {"name": "golang.org/x/net", "version": "v0.60.0", "ecosystem": "Go"},
        # The same IDs as govulncheck's, in osv-scanner's shape (a list, no "fixed"): never an allowlist match.
        "groups": [{"ids": [FIXED, NO_FIX], "max_severity": "7.5"}],
    }],
}]}

NPM_AUDIT = {
    "metadata": {"vulnerabilities": {"critical": 0, "high": 1, "moderate": 0, "low": 0, "info": 0}},
    "vulnerabilities": {"lodash": {"severity": "high", "isDirect": True, "fixAvailable": True,
                                   "via": [{"title": "Prototype pollution"}]}},
}


class VulnReportTest(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.dir = tmp.name
        # Keep the test's report out of the CI job summary.
        env = mock.patch.dict(os.environ)
        env.start()
        self.addCleanup(env.stop)
        os.environ.pop("GITHUB_STEP_SUMMARY", None)

    def write(self, name, content):
        path = os.path.join(self.dir, name)
        with open(path, "w", encoding="utf-8") as f:
            f.write(content if isinstance(content, str) else json.dumps(content))
        return path

    def run_report(self, govuln_findings, allowlist, *, osv=True, npm=True):
        out = os.path.join(self.dir, "report")
        argv = ["--out", out, "--fail-on-called",
                "--govulncheck", "server=" + self.write("govulncheck.json", govulncheck_stream(*govuln_findings)),
                "--allowlist", self.write("allowlist.txt", "".join(f"{v} the reason for {v}\n" for v in allowlist))]
        if osv:
            argv += ["--osv", self.write("osv.json", OSV)]
        if npm:
            argv += ["--npm-audit", "all dependencies=" + self.write("npm.json", NPM_AUDIT),
                     "--npm-audit", "missing=" + os.path.join(self.dir, "absent.json")]
        with contextlib.redirect_stdout(io.StringIO()):
            code = vuln_report.main(argv)
        with open(os.path.join(out, "vulnerabilities.md"), encoding="utf-8") as f:
            md = f.read()
        with open(os.path.join(out, "vulnerabilities.json"), encoding="utf-8") as f:
            result = json.load(f)
        return code, md, result

    def allowlist_line(self, md, vid):
        return next(line for line in md.splitlines() if line.startswith(f"- [{vid}]"))

    def test_allowlist_with_osv_and_npm_results(self):
        # The weekly report's mix, which crashed with KeyError 'id' (osv rows have "ids"; npm audit is a dict).
        code, md, result = self.run_report(
            [(NO_FIX, "", "Verify"), (FIXED, "v0.62.0", "Parse"), (IMPORTED, "v0.62.0", "")], [NO_FIX])
        self.assertEqual(code, 1, "the fixable called vulnerability still fails the job")
        self.assertEqual(sorted(result), ["govulncheck:server", "npm-audit:all dependencies", "osv-scanner"])
        line = self.allowlist_line(md, NO_FIX)
        self.assertNotIn("Stale", line)
        self.assertNotIn("has a fix now", line)
        self.assertIn("1 vulnerabilities in code the server calls (and 1 allowlisted)", md)
        self.assertIn("### osv-scanner (lockfiles)", md)
        self.assertIn("### npm audit: all dependencies", md)

    def test_only_allowlisted_called_passes(self):
        code, md, _ = self.run_report([(NO_FIX, "", "Verify"), (IMPORTED, "v0.62.0", "")], [NO_FIX])
        self.assertEqual(code, 0)
        self.assertIn("0 vulnerabilities in code the server calls (and 1 allowlisted)", md)

    def test_allowlisted_entry_that_has_a_fix_now(self):
        code, md, _ = self.run_report([(FIXED, "v0.62.0", "Parse")], [FIXED])
        self.assertEqual(code, 1, "an entry with a fix is ignored: the vulnerability fails the job")
        self.assertIn("It has a fix now", self.allowlist_line(md, FIXED))

    def test_stale_entry(self):
        # FIXED is only in the osv-scanner results here: that must not count as govulncheck reporting it.
        code, md, _ = self.run_report([(IMPORTED, "v0.62.0", "")], [FIXED])
        self.assertEqual(code, 0)
        line = self.allowlist_line(md, FIXED)
        self.assertIn("Stale", line)
        self.assertNotIn("has a fix now", line)

    def test_govulncheck_only(self):
        # The security workflow's call: no osv-scanner or npm audit input.
        code, md, result = self.run_report([(NO_FIX, "", "Verify")], [NO_FIX], osv=False, npm=False)
        self.assertEqual(code, 0)
        self.assertEqual(list(result), ["govulncheck:server"])
        self.assertNotIn("Stale", self.allowlist_line(md, NO_FIX))


if __name__ == "__main__":
    unittest.main()
