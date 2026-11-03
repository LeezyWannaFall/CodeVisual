#!/usr/bin/env python3
"""Offline production smoke test and sampled process-tree RSS for prepared modules.

Build first, then: python3 scripts/verify_projects.py PATH [PATH ...]
Does not prepare dependencies, download modules, or run analyzed code.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

REPO = Path(__file__).resolve().parents[1]


def fingerprint(root):
    result = {}
    for directory, dirs, files in os.walk(root, followlinks=False):
        dirs[:] = [d for d in dirs if d not in {'.git', 'node_modules', 'vendor'}]
        for name in files:
            p = Path(directory) / name
            if p.is_symlink():
                value = 'symlink:' + os.readlink(p)
            else:
                value = hashlib.sha256(p.read_bytes()).hexdigest()
            result[str(p.relative_to(root))] = value
    return result


def rss_tree(pid):
    """Linux sampled RSS, including Go toolchain children; 0 on other platforms."""
    try:
        status = Path(f'/proc/{pid}/status').read_text()
        rss = next((int(line.split()[1]) for line in status.splitlines() if line.startswith('VmRSS:')), 0)
        children = Path(f'/proc/{pid}/task/{pid}/children').read_text().split()
        return rss + sum(rss_tree(int(child)) for child in children)
    except (OSError, ValueError):
        return 0


def verify(binary, root):
    before = fingerprint(root)
    # Reserve a free port for discovery, then let the CLI own its listener.
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        port = sock.getsockname()[1]
    base = f'http://127.0.0.1:{port}'
    with tempfile.TemporaryDirectory(prefix='codevisual-smoke-') as directory:
        logfile = Path(directory) / 'server.log'
        with logfile.open('w') as log:
            proc = subprocess.Popen([str(binary), 'analyze', str(root), '--no-browser', '--port', str(port)], cwd=directory, stdout=log, stderr=log)
            start = time.monotonic()
            peak = 0
            def get(path):
                with urllib.request.urlopen(base + path, timeout=3) as response:
                    return response.read()
            try:
                while True:
                    peak = max(peak, rss_tree(proc.pid))
                    if proc.poll() is not None:
                        raise RuntimeError(logfile.read_text())
                    try:
                        raw = get('/api/v1/project')
                        break
                    except (OSError, urllib.error.URLError):
                        if time.monotonic() - start > 180:
                            raise TimeoutError('analysis timeout: ' + logfile.read_text())
                        time.sleep(0.05)
                ready = time.monotonic() - start
                info = json.loads(raw)
                sizes = {}
                for level in ['packages', 'directories', 'http', 'database']:
                    body = get('/api/v1/graph?level=' + level)
                    view = json.loads(body)
                    assert len(view['nodes']) <= 200
                    ids = {n['id'] for n in view['nodes']}
                    assert all(e['sourceId'] in ids and e['targetId'] in ids for e in view['edges'])
                    sizes[level] = len(body)
                tree = json.loads(get('/api/v1/tree'))
                for package in (n for n in tree if n['kind'] == 'package'):
                    view = json.loads(get('/api/v1/graph?level=symbols&packageId=' + package['id']))
                    symbol = next((n for n in view['nodes'] if n.get('source')), None)
                    if symbol:
                        source = json.loads(get('/api/v1/files/' + symbol['source']['fileId']))
                        assert source['content']
                        break
                assert b'<html' in get('/').lower()
                html = get('/').decode()
                import re
                for asset in re.findall(r'(?:src|href)="(/assets/[^"]+)"', html):
                    assert get(asset)
                result = {'module': info['project']['modulePath'], 'status': info['status'], 'readySeconds': round(ready, 3), 'sampledPeakTreeRSSMiB': round(peak / 1024, 1), 'statistics': info['statistics'], 'diagnostics': info['diagnosticCount'], 'graphResponseBytes': sizes}
            finally:
                if proc.poll() is None:
                    proc.send_signal(signal.SIGINT)
                    try:
                        proc.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        proc.kill()
                        proc.wait()
            if proc.returncode != 0:
                raise RuntimeError(logfile.read_text())
    assert before == fingerprint(root), f'Analysis changed files in {root}'
    result['inputUnchanged'] = True
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('projects', nargs='*', default=[str(REPO)])
    args = parser.parse_args()
    for project in args.projects:
        print(json.dumps(verify(REPO / 'bin/codevisual', Path(project).resolve()), ensure_ascii=False), flush=True)
