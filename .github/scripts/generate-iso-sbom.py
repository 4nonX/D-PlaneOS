#!/usr/bin/env python3
"""
Generate an SPDX 2.3 SBOM from a Nix closure.

Usage: nix path-info -r result --json | python3 generate-iso-sbom.py <iso-label>

Reads nix path-info JSON from stdin, writes SPDX JSON to stdout.
Each Nix store path becomes one SPDX package with its NAR hash as checksum.
The result reflects the actual closure contents, not an opaque ISO image.
"""
import json, sys, datetime

data = json.load(sys.stdin)
iso_label = sys.argv[1] if len(sys.argv) > 1 else "DPlaneOS Installer ISO"

packages = []
for i, (path, info) in enumerate(data.items()):
    filename = path.split("/")[-1]
    parts = filename.split("-", 1)
    name = parts[1] if len(parts) > 1 else filename
    nar_hash = info.get("narHash", "")
    checksums = []
    if nar_hash:
        # NAR hash format: "sha256:base32..." - strip prefix for SPDX
        raw = nar_hash.replace("sha256:", "")
        checksums = [{"algorithm": "SHA256", "checksumValue": raw}]
    packages.append({
        "SPDXID": f"SPDXRef-Pkg-{i}",
        "name": name,
        "downloadLocation": "https://cache.nixos.org",
        "filesAnalyzed": False,
        "checksums": checksums,
        "comment": f"nix store path: {path}",
    })

doc = {
    "spdxVersion": "SPDX-2.3",
    "dataLicense": "CC0-1.0",
    "SPDXID": "SPDXRef-DOCUMENT",
    "name": iso_label,
    "documentNamespace": (
        f"https://github.com/4nonX/DPlaneOS/sbom/"
        f"{datetime.datetime.utcnow().isoformat()}Z"
    ),
    "documentDescribes": [packages[0]["SPDXID"]] if packages else [],
    "packages": packages,
}

json.dump(doc, sys.stdout, indent=2)
print()  # trailing newline

pkg_count = len(packages)
print(f"Generated SBOM: {pkg_count} packages", file=sys.stderr)
if pkg_count < 10:
    print(
        f"WARNING: only {pkg_count} packages in SBOM for a full NixOS closure - "
        "check that nix path-info -r is reading the correct result symlink",
        file=sys.stderr,
    )
