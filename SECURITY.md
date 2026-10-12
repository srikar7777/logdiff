# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅        |

## Reporting a Vulnerability

**Do not file a public GitHub issue for security vulnerabilities.**

Please report security issues by emailing: security@example.com

Include:
- A description of the vulnerability
- Steps to reproduce it
- The potential impact

You will receive an acknowledgement within 72 hours and a fix timeline within 7 days for confirmed issues.

## Scope

logdiff processes log files from the local filesystem. It does not make outbound network connections during normal operation (except `scripts/download-datasets.sh` which fetches public test data from GitHub).

Potential risk areas:
- **Path traversal**: input file paths are passed directly to `os.Open`
- **Resource exhaustion**: very long log lines or extremely large files
- **Regex DoS**: the tokenizer uses pre-compiled regexps — all patterns have been reviewed for catastrophic backtracking
