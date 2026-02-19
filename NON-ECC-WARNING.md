# ZFS on Non-ECC Hardware: What You Should Know

## Overview

D-PlaneOS detects whether your system has ECC (Error-Correcting Code) RAM and
shows an advisory in the dashboard if it doesn't. This document explains why.

## The Short Version

ZFS protects your data from disk failures and on-disk corruption via checksums.
ZFS **cannot** protect your data from corruption that happens **in RAM** before
it's written to disk. ECC RAM detects and corrects single-bit memory errors;
non-ECC RAM does not.

## Risk Assessment

The risk of a memory bit-flip causing undetected data corruption is **real but
small** for most home users. Research suggests roughly 1 error per GB per year
for consumer RAM (varies widely by hardware and environment).

**What this means in practice:**

| RAM Size | Estimated Errors/Year | Impact |
|----------|----------------------|--------|
| 8 GB     | ~8                   | Very low risk |
| 16 GB    | ~16                  | Low risk |
| 32 GB    | ~32                  | Moderate risk |
| 64 GB    | ~64                  | Consider ECC |
| 128 GB+  | ~128+                | ECC strongly recommended |

More RAM = more surface area for bit flips. Larger storage pools with more
active data in the ARC cache increase the window of exposure.

## What D-PlaneOS Does

- **Detects ECC** via `dmidecode` at startup
- **Shows advisory** on the dashboard (non-blocking — your system works fine)
- **Recommends scrubs** — monthly ZFS scrubs catch corruption early
- **Auto-snapshots** — if corruption is detected, you can roll back

## Recommendations

### For Home/Personal Use (any RAM)
ZFS on non-ECC hardware is **fine** for most users. The risk is comparable to
running any other filesystem on the same hardware. Enable monthly scrubs and
keep backups.

### For Important Data (16+ GB RAM)
Consider ECC if your data is irreplaceable. The cost difference between ECC
and non-ECC DDR4/DDR5 is typically 10-20%.

### For Business/Production (32+ GB RAM)
Use ECC RAM and server-grade hardware. This is standard practice regardless
of filesystem choice.

## What ECC Does NOT Fix

ECC protects against random single-bit memory errors. It does **not** protect
against firmware bugs, software bugs, power loss during writes, or disk
failures. ZFS handles those — that's why the combination of ZFS + ECC is
considered the gold standard.

## Further Reading

- ZFS on Linux FAQ: https://openzfs.github.io/openzfs-docs/
- Cern study on memory errors in production: "DRAM Errors in the Wild"
