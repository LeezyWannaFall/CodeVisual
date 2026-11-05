"""Fail if a frontend rebuild changed, removed, or added embedded assets."""
import subprocess
import sys

result = subprocess.run(
    ["git", "status", "--porcelain", "--untracked-files=all", "--", "internal/server/webdist"],
    check=True, capture_output=True, text=True,
)
if result.stdout:
    print("Embedded frontend differs from the committed build. Run make frontend and commit webdist.")
    print(result.stdout)
    sys.exit(1)
