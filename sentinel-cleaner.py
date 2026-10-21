#!/usr/bin/env python3
"""
DPRK Config-Worm Remediation & Sentinel Tool

Modes:
  1. Scan (Dry Run):
     python3 sentinel-cleaner.py /path/to/code --scan

  2. Local Clean:
     python3 sentinel-cleaner.py /path/to/code --clean

  3. Clean + Commit + Force Push:
     python3 sentinel-cleaner.py /path/to/code --clean --commit --force-push

  4. Continuous Sentinel (Watch & Auto-Block):
     python3 sentinel-cleaner.py /path/to/code --watch [--interval 2]
"""

import argparse
import datetime
import os
import re
import subprocess
import sys
import time
from pathlib import Path

# Match malicious payload appended to config files
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
SKIP_DIRS = {".git", "node_modules", ".venv", "dist", "build", ".next", ".cache"}


def run_git(repo_dir: Path, *args) -> tuple[int, str, str]:
    res = subprocess.run(
        ["git", *args],
        cwd=repo_dir,
        capture_output=True,
        text=True,
    )
    return res.returncode, res.stdout.strip(), res.stderr.strip()


def find_git_root(path: Path) -> Path | None:
    curr = path.resolve()
    for parent in [curr, *curr.parents]:
        if (parent / ".git").is_dir():
            return parent
    return None


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


def scan_directory(target_dir: Path) -> tuple[dict[Path, list[Path]], list[Path]]:
    infected_by_repo: dict[Path, list[Path]] = {}
    standalone_infected: list[Path] = []

    for root, dirs, files in os.walk(target_dir):
        dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
        for file in files:
            path = Path(root) / file
            if path.suffix in TARGET_EXTENSIONS and scan_file(path):
                git_root = find_git_root(path)
                if git_root:
                    infected_by_repo.setdefault(git_root, []).append(path)
                else:
                    standalone_infected.append(path)

    return infected_by_repo, standalone_infected


def mode_scan(target_dir: Path):
    print(f"\n[*] Scanning: {target_dir}")
    infected_by_repo, standalone = scan_directory(target_dir)
    total = sum(len(v) for v in infected_by_repo.values()) + len(standalone)

    if total == 0:
        print("[+] Scan clean: No infected files found.")
        return

    print(f"\n[!] ALERT: Found {total} infected file(s):\n")
    for repo, files in infected_by_repo.items():
        _, branch, _ = run_git(repo, "branch", "--show-current")
        print(f"📁 Repository: {repo.name} (branch: '{branch}') [{repo}]")
        for f in files:
            print(f"   └── ⚠️  Infected: {f.relative_to(repo)}")
    for f in standalone:
        print(f"📁 Non-git file:\n   └── ⚠️  Infected: {f}")


def mode_clean(target_dir: Path, do_commit: bool, do_force_push: bool):
    print(f"\n[*] Running Clean Routine on: {target_dir}")
    infected_by_repo, standalone = scan_directory(target_dir)
    total = sum(len(v) for v in infected_by_repo.values()) + len(standalone)

    if total == 0:
        print("[+] No infected files to clean.")
        return

    print(f"[*] Cleaning {total} file(s)...")
    for repo, files in infected_by_repo.items():
        print(f"\n--- Cleaning Repo: {repo.name} ---")
        for f in files:
            ok, snippet = clean_file(f)
            if ok:
                print(f"  [✓] Sanitized: {f.relative_to(repo)}")
                print(f"      Removed: {snippet}")
            else:
                print(f"  [✗] Error cleaning: {f.relative_to(repo)} ({snippet})")

    for f in standalone:
        ok, _ = clean_file(f)
        print(f"  [✓] Sanitized non-git file: {f}")

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


def mode_watch(target_dir: Path, interval: float):
    print("\n" + "=" * 65)
    print(f"[*] SENTINEL ACTIVE: Monitoring {target_dir}")
    print(f"[*] Polling interval: {interval}s. Auto-neutralization: ENABLED.")
    print("[*] Press Ctrl+C to stop.")
    print("=" * 65 + "\n")

    try:
        while True:
            infected_by_repo, standalone = scan_directory(target_dir)
            all_files = [f for files in infected_by_repo.values() for f in files] + standalone

            if all_files:
                ts = datetime.datetime.now().strftime("%Y-%m-%d %H:%M:%S")
                # Trigger audible terminal bell
                sys.stdout.write("\a")
                sys.stdout.flush()

                print(f"\n[🚨 RE-INFECTION DETECTED] {ts}")
                for f in all_files:
                    proc_info = identify_writing_process(f)
                    ok, snippet = clean_file(f)
                    if ok:
                        print(f"  [SHIELD BLOCKED & SANITIZED] {f}")
                        print(f"    └── Suspect process: {proc_info}")
                        print(f"    └── Stripped payload: {snippet[:70]}")
                    else:
                        print(f"  [ERROR SANITIZING] {f}")

            time.sleep(interval)
    except KeyboardInterrupt:
        print("\n[*] Sentinel stopped by user.")


def main():
    parser = argparse.ArgumentParser(description="DPRK Malware Scanner, Cleaner & Sentinel")
    parser.add_argument("path", type=str, help="Target directory to scan/monitor")
    parser.add_argument("--scan", action="store_true", help="Mode 1: Dry run scan only")
    parser.add_argument("--clean", action="store_true", help="Mode 2: Clean files locally")
    parser.add_argument("--commit", action="store_true", help="Commit clean changes")
    parser.add_argument("--force-push", action="store_true", help="Mode 3: Force push clean commits")
    parser.add_argument("--watch", action="store_true", help="Mode 4: Continuous active sentinel")
    parser.add_argument("--interval", type=float, default=2.0, help="Watch interval in seconds (default: 2)")

    args = parser.parse_args()
    target_dir = Path(args.path).resolve()

    if not target_dir.exists():
        print(f"Error: Target path '{target_dir}' does not exist.")
        sys.exit(1)

    if args.watch:
        mode_watch(target_dir, args.interval)
    elif args.clean or args.commit or args.force_push:
        mode_clean(target_dir, args.commit, args.force_push)
    else:
        mode_scan(target_dir)


if __name__ == "__main__":
    main()
