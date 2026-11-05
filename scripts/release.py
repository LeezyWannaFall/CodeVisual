"""Build standalone release archives using only Go and the Python standard library."""
import argparse
import hashlib
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent
TARGETS = [(system, arch) for system in ("linux", "darwin", "windows") for arch in ("amd64", "arm64")]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--version", required=True, help="Release tag, e.g. v0.2.0")
    parser.add_argument("--output", type=Path, default=ROOT / "dist")
    args = parser.parse_args()
    if not re.fullmatch(r"v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?", args.version):
        parser.error("version must be a v-prefixed release version, e.g. v0.2.0")
    args.output.mkdir(parents=True, exist_ok=True)
    checksums = []
    for system, arch in TARGETS:
        name = f"codevisual_{args.version}_{system}_{arch}"
        binary_name = "codevisual.exe" if system == "windows" else "codevisual"
        env = dict(os.environ, GOOS=system, GOARCH=arch, CGO_ENABLED="0", GOTOOLCHAIN="local")
        with tempfile.TemporaryDirectory(prefix="codevisual-release-") as temp:
            binary = Path(temp) / binary_name
            subprocess.run([
                "go", "build", "-mod=readonly", "-trimpath", "-buildvcs=false",
                "-ldflags", f"-s -w -X codevisual/internal/cli.Version={args.version}",
                "-o", str(binary), "./cmd/codevisual",
            ], cwd=ROOT, env=env, check=True)
            files = [(binary, binary_name), (ROOT / "README.md", "README.md"), (ROOT / "LICENSE", "LICENSE")]
            archive = args.output / (name + (".zip" if system == "windows" else ".tar.gz"))
            if system == "windows":
                with zipfile.ZipFile(archive, "w", compression=zipfile.ZIP_DEFLATED) as package:
                    for source, target in files:
                        package.write(source, target)
            else:
                with tarfile.open(archive, "w:gz") as package:
                    for source, target in files:
                        package.add(source, arcname=target)
            checksums.append(f"{hashlib.sha256(archive.read_bytes()).hexdigest()}  {archive.name}\n")
            print(archive.name, flush=True)
    (args.output / "SHA256SUMS").write_text("".join(checksums), encoding="utf-8")


if __name__ == "__main__":
    main()
