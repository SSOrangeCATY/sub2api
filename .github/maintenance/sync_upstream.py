#!/usr/bin/env python3
"""Merge official upstream into custom work; never update production."""
import argparse
from datetime import datetime, timezone
from pathlib import Path
import subprocess
import sys
import re

ROOT = Path(__file__).resolve().parents[2]

def run(*args, capture=False):
    return subprocess.run(args, cwd=ROOT, check=True, text=True,
                          stdout=subprocess.PIPE if capture else None).stdout

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--publish', action='store_true', help='Push sync branch and open custom/main PR')
    args = parser.parse_args()
    if run('git', 'status', '--porcelain', capture=True).strip():
        sys.exit('Working tree must be clean; finish any existing merge first.')
    if args.publish:
        run('gh', 'auth', 'status')
    origin = run('git', 'remote', 'get-url', 'origin', capture=True).strip()
    match = re.fullmatch(r'(?:https://github[.]com/|git@github[.]com:)([A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+?)(?:[.]git)?', origin)
    if not match:
        sys.exit('origin must be a GitHub HTTPS or SSH repository URL.')
    repository = match.group(1)
    run('git', 'fetch', 'origin', 'custom/main')
    run('git', 'fetch', 'https://github.com/Wei-Shaw/sub2api.git',
        'refs/heads/main:refs/remotes/upstream/main')
    sha = run('git', 'rev-parse', 'refs/remotes/upstream/main', capture=True).strip()
    branch = 'sync/upstream-' + datetime.now(timezone.utc).strftime('%Y%m%d') + '-' + sha[:10]
    run('git', 'switch', '-c', branch, 'origin/custom/main')
    # Normal merge only: a conflict intentionally stops with its state intact.
    run('git', 'merge', '--no-ff', '--no-edit', sha)
    if not args.publish:
        print('Merge prepared. Run checks, then git push -u origin HEAD and gh pr create --repo ' + repository + ' --base custom/main.')
        return
    run('git', 'push', 'origin', sha + ':refs/heads/upstream/main')
    run('git', 'push', '-u', 'origin', branch)
    run('gh', 'pr', 'create', '--repo', repository, '--base', 'custom/main', '--head', branch,
        '--title', 'chore: merge upstream ' + sha[:10] + ' with local patches',
        '--body', 'Merge official Wei-Shaw/sub2api main into local customizations. Review conflicts and all checks before merging; production requires a separate custom/main PR.')

if __name__ == '__main__':
    try:
        main()
    except subprocess.CalledProcessError as exc:
        sys.exit(exc.returncode)
