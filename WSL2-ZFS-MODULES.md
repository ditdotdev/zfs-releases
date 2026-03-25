# Plan: ZFS Module Loading via insmod for WSL2 + Package Manager Cleanup

## Context

We proved that on modern WSL2 (kernel >= 6.6.36.3 with `CONFIG_MODULES=y`), ZFS kernel modules can be loaded via `insmod` from a privileged Docker container — no `.wslconfig` changes, no VHD packaging, no kernel replacement needed.

**What we proved:**
1. Built `zfs.ko` + `spl.ko` against WSL2 kernel source in Ubuntu distro
2. `insmod spl.ko && insmod zfs.ko` works from WSL2 directly
3. `docker run --privileged -v /path/to/modules:/zfs-modules ubuntu insmod /zfs-modules/spl.ko && insmod /zfs-modules/zfs.ko` works from a container
4. `apt install zfsutils-linux` provides userland tools (zpool, zfs commands)

**Why this matters:** The current launch container's `modprobe zfs` fails on WSL2 because `/lib/modules/<kernel>/` has no ZFS `.ko` — Microsoft doesn't ship kernel headers, so DKMS can't compile them, and `apt install zfsutils-linux` only provides userland tools, not the kernel module.

**The solution:** Pre-build ZFS `.ko` modules for each WSL2 kernel version, distribute via zfs-releases S3 bucket, and have the launch container `insmod` them when `modprobe` fails.

## WSL2 Release / Kernel Compatibility Matrix

Only WSL releases with `CONFIG_MODULES=y` kernels are supported (>= WSL 2.5.1 / kernel 6.6.36.3):

| WSL Version | Release Date | Kernel Version | CONFIG_MODULES | Notes |
|------------|-------------|----------------|----------------|-------|
| 2.5.1 | 2025-03-12 | 6.6.75.x | y | First release with modules VHD support |
| 2.5.7 | 2025-04-24 | 6.6.75.x | y | |
| 2.5.9 | 2025-06-10 | 6.6.87.1 | y | |
| 2.5.10 | 2025-08-06 | 6.6.87.2 | y | |
| 2.6.1 | 2025-08-07 | 6.6.87.x | y | |
| 2.6.2 | 2025-10-13 | 6.6.87.x | y | |
| 2.6.3 | 2025-12-12 | 6.6.87.2+ | y | Latest stable |

**Unsupported:** WSL 2.3.x, 2.4.x (ship with 5.15.x kernels, no CONFIG_MODULES=y). Users on these versions should run `wsl --update` to upgrade.

**Key dates:**
- 2024-07-02: `linux-msft-wsl-6.6.36.3` — first kernel with CONFIG_MODULES=y
- 2025-02-11: `linux-msft-wsl-6.6.75.1` — modules distributed as VHD
- 2025-03-12: WSL 2.5.1 — first WSL release shipping modules-as-VHD

## New ZFS Loading Chain (4 steps)

```
Step 1: ZFS already loaded?        (KEEP - check_running_zfs)
  |
Step 2: Host system modules?       (KEEP - load_zfs via modprobe)
  |
Step 3: Package manager install?   (KEEP - apt install zfsutils-linux + modprobe)
  |
Step 4: insmod prebuilt modules?   (NEW - for WSL2 and other non-standard kernels)
         a. Download spl.ko + zfs.ko for $(uname -r) from zfs-releases S3 bucket
         b. insmod spl.ko && insmod zfs.ko
         c. Install zfsutils-linux for userland if not present
```

Step 3 works on standard Linux (GH Actions, bare metal Ubuntu, etc.).
Step 4 is the WSL2-specific fallback when modprobe fails because no matching .ko exists in /lib/modules/.

## Repos and Branches

| Repo | Branch | Scope |
|------|--------|-------|
| `datadatdat-server` | `simplify/zfs-insmod-loading` | zfs.sh changes, launch script, BATS tests |
| `zfs-releases` | `feature/wsl2-module-builds` | CI workflow, S3 publishing, plan .MD |
| `datadatdat` | `update/install-flow-docs` | DATADATDAT_INSTALL_FLOW.md update |

## Files to Modify

### 1. `datadatdat-server/server/src/scripts/zfs.sh`

**Remove:**
- `get_zfs_build_version()` — hardcoded S3/build version
- `load_precompiled_zfs()` — old S3 download logic for full module tarballs
- `compile_and_load_zfs()` — zfs-builder Docker invocation
- Any S3 URL construction for the old module format

**Add:**
- `install_zfs_packages()` — Pre-flight checks + `apt install zfsutils-linux` + `modprobe zfs`
  - Detect package manager (apt/dnf/pacman/apk)
  - Check CONFIG_MODULES=y
  - Check ZFS packages in repos
  - Install + modprobe
- `insmod_prebuilt_zfs()` — Download + insmod for WSL2/non-standard kernels
  - Download `spl.ko` + `zfs.ko` for `$(uname -r)` from zfs-releases S3 bucket
  - Cache to `/var/lib/datadatdat/data/modules/$(uname -r)/`
  - `insmod spl.ko && insmod zfs.ko`
  - Install `zfsutils-linux` for userland tools if not already present

**Keep unchanged:**
- `is_zfs_loaded()` (includes built-in ZFS detection from PR #91)
- `check_running_zfs()` — Step 1
- `load_zfs()` — Step 2
- All pool management functions
- All verification/sanity functions
- `unload_zfs()`, `unmount_filesystems()`

### 2. `datadatdat-server/server/src/scripts/launch`

**Remove:**
- Docker Desktop LinuxKit special case
- `COMPILED_MODULES` variable references

**Update fallback chain:**
```bash
if ! check_running_zfs &&
   ! load_zfs $SYSTEM_MODULES system $INSTALL_DIR &&
   ! install_zfs_packages $INSTALL_DIR &&
   ! insmod_prebuilt_zfs $INSTALL_DIR; then
    log_error "Failed to load ZFS. See docs for manual installation."
fi
```

### 3. `zfs-releases` — CI pipeline for building .ko modules

**New workflow: `.github/workflows/wsl2-modules.yml`**

Builds `spl.ko` + `zfs.ko` for each supported WSL2 kernel version:
1. Clone WSL2-Linux-Kernel source for target version
2. `make olddefconfig && make modules_prepare && make -j$(nproc) modules`
3. Clone OpenZFS, `./configure --with-linux=... && make -j$(nproc)`
4. Package `spl.ko` + `zfs.ko` into tarball
5. Upload to zfs-releases S3 bucket
6. Naming: `zfs-<zfs_version>-<kernel_version>.tar.gz`

**Initial build matrix (expand later):**
- WSL2 kernel: 6.6.87.2 only (test against current install first)
- ZFS version: latest stable (currently 2.4.1)

**Trigger:** Manual dispatch initially. Add automated WSL2 kernel release detection later.

**New plan .MD:** `WSL2-ZFS-MODULES.md` in zfs-releases repo (this plan, committed to branch)

### 4. `datadatdat/DATADATDAT_INSTALL_FLOW.md`

Update the ZFS Module Loading section to reflect the new 4-step chain and the insmod approach.

### 5. Cleanup — Remove from ecosystem

| Item | Action |
|------|--------|
| S3 bucket old precompiled module tarballs | Deprecate old format; new format is per-kernel .ko pairs |
| `datadatdat/zfs-builder` WSL path (`src/wsl.sh`) | Keep on experiment branch only |
| `datadatdat/docker-desktop-zfs-kernel` images | Deprecate |
| zfs-builder bzImage CI workflows | Keep on experiment branch only |
| 30-minute compile-from-source fallback | Removed from launch chain |

### 6. Issue #79

Update with current status. Do NOT close — still needs Phase 4 (d3 install detecting WSL2 + offering kernel upgrade).

## TDD Approach

### BATS tests in `datadatdat-server/server/src/scripts/tests/`

1. **`install_zfs_packages()`** — detect pkg manager, pre-flight checks, error messages
2. **`insmod_prebuilt_zfs()`** — download logic, insmod calls, fallback behavior
3. **Removed functions are gone** — verify `compile_and_load_zfs`, `load_precompiled_zfs` don't exist
4. **New launch chain** — all 4 steps, error propagation

### Local verification

1. BATS tests pass
2. Build server Docker image locally
3. `d3 install` on WSL2 with local image — ZFS loads via insmod path
4. `d3 install` on standard Linux — ZFS loads via package manager path
5. Pre-flight errors: clear messages for CONFIG_MODULES=n, missing repos

## PRs Required

| PR | Repo | Branch | Description |
|----|------|--------|-------------|
| 1 | datadatdat-server | `simplify/zfs-insmod-loading` | zfs.sh simplification + insmod support + BATS tests |
| 2 | zfs-releases | `feature/wsl2-module-builds` | CI workflow for .ko builds + plan .MD |
| 3 | datadatdat | `update/install-flow-docs` | DATADATDAT_INSTALL_FLOW.md update |

## Development Order

1. Create branches in all 3 repos
2. Write failing BATS tests for new functions (datadatdat-server)
3. Implement `install_zfs_packages()` in zfs.sh
4. Implement `insmod_prebuilt_zfs()` in zfs.sh
5. Remove old S3/compile functions from zfs.sh
6. Update launch script fallback chain
7. Update existing BATS tests for removed functions
8. Build and test Docker image locally against WSL2 (kernel 6.6.87.2)
9. Set up zfs-releases CI for .ko module builds (6.6.87.2 only)
10. Upload test modules to S3, full E2E test on WSL2
11. Update DATADATDAT_INSTALL_FLOW.md
12. Create PRs for all 3 repos
13. Update issue #79
