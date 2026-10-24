#!/usr/bin/env python3
"""Update the same-repository Homebrew tap from an immutable stable tag archive."""
import hashlib
from pathlib import Path
import re
import sys
import urllib.request


def update(tag, archive, formula):
    if not re.fullmatch(r"v\d+\.\d+\.\d+", tag):
        raise ValueError("A stable vMAJOR.MINOR.PATCH tag is required")
    url = f"https://github.com/coffeedeveloper/cpc/archive/refs/tags/{tag}.tar.gz"
    digest = hashlib.sha256(archive).hexdigest()
    source = formula.read_text()
    source = re.sub(r'^  (?:url|sha256) .*\n', '', source, flags=re.M)
    source = source.replace('  # Stable URL and checksum are added automatically by the release workflow.\n', '')
    marker = '  homepage "https://github.com/coffeedeveloper/cpc"\n'
    if marker not in source:
        raise ValueError("Formula homepage marker missing")
    formula.write_text(source.replace(marker, marker + f'  url "{url}"\n  sha256 "{digest}"\n'))


if __name__ == "__main__":
    tag = sys.argv[1]
    if not re.fullmatch(r"v\d+\.\d+\.\d+", tag):
        sys.exit("A stable vMAJOR.MINOR.PATCH tag is required")
    url = f"https://github.com/coffeedeveloper/cpc/archive/refs/tags/{tag}.tar.gz"
    with urllib.request.urlopen(url, timeout=60) as response:
        archive = response.read()
    update(tag, archive, Path("Formula/cpc.rb"))
