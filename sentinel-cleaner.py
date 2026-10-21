#!/usr/bin/env python3
"""
DPRK Config-Worm Remediation & Sentinel Tool

Modes:
  1. Scan (Dry Run):
     sentinel-cleaner.py [root_dir] --scan

  2. Local Clean:
     sentinel-cleaner.py [root_dir] --clean

  3. Clean + Commit + Force Push:
     sentinel-cleaner.py [root_dir] --clean --commit --force-push

  4. Continuous Sentinel (Watch & Auto-Block):
     sentinel-cleaner.py [root_dir] --watch [--interval 2]

Defaults to ~/code if no root directory is specified.
Automatically walks the root directory to discover all Git repositories.
"""

import argparse
import datetime
import os
import re
import subprocess
import sys
import time
from pathlib import Path

PAYLOAD_PATTERN = re.compile(
    r"[ \t]{100,}(?:import\s*\{[^}]*createRequire.*)?global\.o=['\"][^'\"]+['\"].*$",
    re.DOTALL,
)

SIGNATURES = [
    b"mdmnideb%jtanef",
    b"global.o='8-hhn",
    b"global.o=\"8-hhn",
]

TARGET_EXTENSIONS = {".js", ".mjs", ".cjs", ".ts"}
SKIP_DIRS = {".git", "node_modules", ".venv", "venv", "dist", "build", ".next", ".cache", "vendor", "target"}


def run_git(repo_dir: Path, *args) -> tuple[int, str, str]:
    res = subprocess.run(
        ["git", *args],
        cwd=repo_dir,
        capture_output=True,
        text=True,
    )
    return res.returncode, res.stdout.strip(), res.stderr.strip()


def discover_git_repos(root_dir: Path) -> list[Path]:
    """Walks the root directory and discovers all Git repositories."""
    repos = []
    # If the root_dir itself is a git repo
    if (root_dir / ".git").is_dir():
        repos.append(root_dir)
        return repos

    for dirpath, dirnames, _ in os.walk(root_dir):
        if ".git" in dirnames:
            repos.append(Path(dirpath))
            dirnames.remove(".git")  # Don't recurse into .git itself
        # Don't recurse into unwanted huge directories
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
    return sorted(repos)


def scan_file(file_path: Path) -> bool:
    try:
        data = file_path.read_bytes()
        return any(sig in data for sig in SIGNATURES)
    except (PermissionError, FileNotFoundError, OSError):
        return False


def clean_file(file_path: Path) -> tuple[bool, str]:
    try:
        content = file_path.read_text(encoding="utf-8", errors="ignore")

        # Strip injected require shims
        cleaned = re.sub(
            r"import\s*\{\s*createRequire\s*\}\s*from\s*['\"]node:module['\"];\s*\nconst\s+require\s*=\s*createRequire\(import\.meta\.url\);\s*\n?",
            "",
            content,
        )

        match = PAYLOAD_PATTERN.search(cleaned)
        snippet = match.group(0)[:80] + "..." if match else "(obfuscated block)"

        # Strip payload and trailing spaces
        cleaned = re.sub(PAYLOAD_PATTERN, "", cleaned).rstrip() + "\n"
        file_path.write_text(cleaned, encoding="utf-8")
        return True, snippet.strip()
    except Exception as e:
        return False, str(e)


def identify_writing_process(file_path: Path) -> str:
    """Attempt to identify which process is modifying the file (macOS/Linux)."""
    try:
        proc = subprocess.run(
            ["lsof", "-t", str(file_path)],
            capture_output=True,
            text=True,
            timeout=1,
        )
        pids = proc.stdout.strip().split()
        if pids:
            details = []
            for pid in pids:
                ps = subprocess.run(
                    ["ps", "-p", pid, "-o", "pid,comm,args="],
                    capture_output=True,
                    text=True,
                )
                details.append(ps.stdout.strip())
            return " | ".join(details)
    except Exception:
        pass
    return "Unknown process"


def scan_repo(repo_dir: Path) -> list[Path]:
    """Scans all JS/TS config files in a given Git repository."""
    infected = []
    for root, dirs, files in os.walk(repo_dir):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for file in files:
            path = Path(root) / file
            if path.suffix in TARGET_EXTENSIONS and scan_file(path):
                infected.append(path)
    return infected


def scan_all_repos(repos: list[Path]) -> dict[Path, list[Path]]:
    infected_by_repo = {}
    for repo in repos:
        inf = scan_repo(repo)
        if inf:
            infected_by_repo[repo] = inf
    return infected_by_repo


def mode_scan(root_dir: Path, repos: list[Path], verbose: bool):
    print(f"\n[*] Root directory: {root_dir}")
    print(f"[*] Discovered {len(repos)} Git repositories.")
    if verbose:
        for r in repos:
            print(f"    - {r.relative_to(root_dir) if r != root_dir else r.name}")

    infected_by_repo = scan_all_repos(repos)
    total = sum(len(v) for v in infected_by_repo.values())

    if total == 0:
        print("\n[+] Scan clean: All discovered Git repositories are uninfected.")
        return

    print(f"\n[!] ALERT: Found {total} infected file(s) across {len(infected_by_repo)} repo(s):\n")
    for repo, files in infected_by_repo.items():
        _, branch, _ = run_git(repo, "branch", "--show-current")
        print(f"📁 Repository: {repo.name} (branch: '{branch}') [{repo}]")
        for f in files:
            print(f"   └── ⚠️  Infected: {f.relative_to(repo)}")


def mode_clean(root_dir: Path, repos: list[Path], do_commit: bool, do_force_push: bool):
    print(f"\n[*] Running Clean Routine across {len(repos)} Git repos under: {root_dir}")
    infected_by_repo = scan_all_repos(repos)
    total = sum(len(v) for v in infected_by_repo.values())

    if total == 0:
        print("[+] No infected files found to clean.")
        return

    print(f"[*] Cleaning {total} infected file(s)...")
    for repo, files in infected_by_repo.items():
        print(f"\n--- Cleaning Repo: {repo.name} ---")
        for f in files:
            ok, snippet = clean_file(f)
            if ok:
                print(f"  [✓] Sanitized: {f.relative_to(repo)}")
                print(f"      Removed payload: {snippet}")
            else:
                print(f"  [✗] Error cleaning: {f.relative_to(repo)} ({snippet})")

    if do_commit:
        print("\n--- Committing Clean Files ---")
        for repo in infected_by_repo.keys():
            run_git(repo, "add", "-u")
            code, out, err = run_git(
                repo, "commit", "-m", "fix: remove unauthorized malicious payload from config"
            )
            if code == 0:
                print(f"  [✓] Created clean commit in {repo.name}")
            else:
                print(f"  [-] Commit skipped for {repo.name}: {err or out}")

    if do_force_push:
        print("\n--- Force-Push Routine ---")
        confirm = input("[CAUTION] Force-push clean branches to origin? [y/N]: ")
        if confirm.lower() != "y":
            print("[*] Push aborted.")
            return

        for repo in infected_by_repo.keys():
            _, branch, _ = run_git(repo, "branch", "--show-current")
            if not branch:
                print(f"  [✗] Skipped {repo.name} (detached HEAD)")
                continue

            print(f"  [>>] Force-pushing {repo.name} -> origin/{branch}...")
            code, out, err = run_git(repo, "push", "--force-with-lease", "origin", branch)
            if code == 0:
                print(f"  [✓] Pushed successfully: {repo.name}")
            else:
                print(f"  [✗] Failed pushing {repo.name}: {err or out}")


def mode_watch(root_dir: Path, interval: float):
    repos = discover_git_repos(root_dir)
    print("\n" + "=" * 65)
    print(f"[*] SENTINEL ACTIVE: Monitoring {len(repos)} Git repos under {root_dir}")
    print(f"[*] Polling interval: {interval}s. Auto-neutralization: ENABLED.")
    print("[*] Automatically detects newly cloned repositories.")
    print("[*] Press Ctrl+C to stop.")
    print("=" * 65 + "\n")

    tick = 0
    try:
        while True:
            # Re-discover repos periodically (every 10 cycles) in case new repos are cloned
            if tick % 10 == 0:
                repos = discover_git_repos(root_dir)

            infected_by_repo = scan_all_repos(repos)

            if infected_by_repo:
                ts = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                sys.stdout.write("\a")
                sys.stdout.flush()

                print(f"\n[🚨 RE-INFECTION DETECTED] {ts}")
                for repo, files in infected_by_repo.items():
                    for f in files:
                        proc_info = identify_writing_process(f)
                        ok, snippet = clean_file(f)
                        if ok:
                            print(f"  [SHIELD BLOCKED & SANITIZED] {f.relative_to(root_dir)}")
                            print(f"    └── Repo: {repo.name}")
                            print(f"    └── Suspect process: {proc_info}")
                            print(f"    └── Stripped payload: {snippet[:70]}")
                        else:
                            print(f"  [ERROR SANITIZING] {f.relative_to(root_dir)}")

            time.sleep(interval)
            tick += 1
    except KeyboardInterrupt:
        print("\n[*] Sentinel stopped by user.")


def resolve_default_root() -> Path:
    code_dir = Path("~/code").expanduser().resolve()
    if code_dir.is_dir():
        return code_dir
    return Path.cwd().resolve()


def main():
    parser = argparse.ArgumentParser(
        description="DPRK Malware Scanner, Cleaner & Sentinel (Git Repo Walker)"
    )
    parser.add_argument(
        "path",
        nargs="?",
        default=None,
        help="Root directory to walk for Git repos (defaults to ~/code)",
    )
    parser.add_argument("--scan", action="store_true", help="Mode 1: Dry run scan only")
    parser.add_argument("--clean", action="store_true", help="Mode 2: Clean files locally")
    parser.add_argument("--commit", action="store_true", help="Commit clean changes")
    parser.add_argument("--force-push", action="store_true", help="Mode 3: Force push clean commits")
    parser.add_argument("--watch", action="store_true", help="Mode 4: Continuous active sentinel")
    parser.add_argument("--interval", type=float, default=2.0, help="Watch interval in seconds (default: 2)")
    parser.add_argument("-v", "--verbose", action="store_true", help="List all discovered Git repos")

    args = parser.parse_args()

    if args.path:
        root_dir = Path(args.path).expanduser().resolve()
    else:
        root_dir = resolve_default_root()

    if not root_dir.exists():
        print(f"Error: Root directory '{root_dir}' does not exist.")
        sys.exit(1)

    if args.watch:
        mode_watch(root_dir, args.interval)
    else:
        repos = discover_git_repos(root_dir)
        if args.clean or args.commit or args.force_push:
            mode_clean(root_dir, repos, args.commit, args.force_push)
        else:
            mode_scan(root_dir, repos, args.verbose)


if __name__ == "__main__":
    main()
