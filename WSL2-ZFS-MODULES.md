# Plan: ZFS Module Loading via insmod for WSL2

**Tracking issue:** https://github.com/datadatdat/datadatdat/issues/79

## Context

We proved that on modern WSL2 (kernel >= 6.6.36.3 with `CONFIG_MODULES=y`), ZFS kernel modules can be loaded via `insmod` from a privileged Docker container — no `.wslconfig` changes, no VHD packaging, no kernel replacement needed.

**What we proved:**
1. Built `zfs.ko` + `spl.ko` against WSL2 kernel source in Ubuntu distro
2. `insmod spl.ko && insmod zfs.ko` works from WSL2 directly
3. `docker run --privileged -v /path/to/modules:/zfs-modules ubuntu insmod /zfs-modules/spl.ko && insmod /zfs-modules/zfs.ko` works from a container
4. `apt install zfsutils-linux` provides userland tools (zpool, zfs commands)
5. Full E2E tests pass (147/147) on WSL2 with stock Microsoft kernel 6.6.87.2
6. ZFS modules download from S3 and load automatically via the launch container's 4-step fallback chain

**Key discovery: ZFS kernel module version must match userland tools version.** Mismatched versions (e.g., kmod 2.4.1 + userland 2.2.2) cause `zpool create` to fail with misleading errors.

**Key discovery: WSL2 `zpool create` fails with file-backed vdevs.** Must use `losetup` to create a loop device first, then pass the loop device to `zpool create`.

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

## ZFS Loading Chain (4 steps)

```
Step 1: ZFS already loaded?        (check_running_zfs)
  |
Step 2: Host system modules?       (load_zfs via modprobe)
  |
Step 3: Package manager install?   (apt/dnf/pacman + modprobe)
  |
Step 4: insmod prebuilt modules?   (download from S3 + insmod)
         a. Download spl.ko + zfs.ko for $(uname -r) from S3
         b. insmod spl.ko && insmod zfs.ko
         c. Install zfsutils-linux for userland if not present
```

## Completed Work

### Phase 1: datadatdat-server changes (PR #98 — MERGED)
- Simplified zfs.sh: 4-step chain replacing old 5-step chain
- Added `install_zfs_packages()` with package manager detection
- Added `insmod_prebuilt_zfs()` with S3 download + insmod
- Fixed `zpool create` on WSL2 with loop device fallback
- Fixed all shellcheck warnings in zfs.sh and launch
- Removed `get-userland` dependency on deleted `get_zfs_build_version`
- 28/28 BATS tests pass
- 147/147 E2E tests pass on WSL2

### Phase 2: Install flow documentation (PR #85 — MERGED)
- Updated DATADATDAT_INSTALL_FLOW.md with 4-step chain
- Added WSL2 notes for loop device pool creation
- Removed zfs-builder from Docker images table

### Phase 3: CI workflow for WSL2 module builds (PR #64 — MERGED)
- Added `wsl2-modules.yml` workflow
- Successfully built and published ZFS 2.2.2 modules for kernel 6.6.87.2 to S3
- Full E2E validated: fresh `d3 install` downloads from S3 and loads via insmod

## Remaining Work

### Phase 4: Build latest ZFS with kernel modules + userland tools

**Goal:** Build the latest stable ZFS version from source, producing both kernel modules (`spl.ko` + `zfs.ko`) and userland tools (`zpool`, `zfs`, libs). Package together in a single archive per kernel version.

**Archive naming convention:**
```
zfs-<ZFS_VERSION>-modules-<KREL>.tar.gz
```
Example: `zfs-2.3.6-modules-6.6.87.2-microsoft-standard-WSL2.tar.gz`

**ZFS version selection:** Query the latest stable release from `openzfs/zfs` GitHub releases API. As of 2026-03-25, the latest stable releases are:
- `zfs-2.3.6` (latest 2.3.x LTS)
- `zfs-2.4.1` (latest 2.4.x)

Use the latest 2.x stable release. Both kernel modules and userland must be built from the same version.

**Archive contents:**
```
spl.ko           # Kernel module
zfs.ko           # Kernel module
sbin/zpool       # Userland
sbin/zfs         # Userland
lib/              # Shared libraries (libzfs, libnvpair, etc.)
VERSION          # Text file: ZFS_VERSION=2.3.6 KREL=6.6.87.2-microsoft-standard-WSL2
```

**Update `insmod_prebuilt_zfs()` in zfs.sh** to:
1. Download the combined archive (not just .ko files)
2. `insmod` the kernel modules
3. Install userland tools from the archive (not from apt)
4. Ensures version match between kernel module and userland

### Phase 5: Build matrix for all WSL2 kernel versions

**Goal:** Build ZFS modules for all active WSL2 kernel versions in parallel.

**Build matrix:**
```yaml
strategy:
  fail-fast: false
  matrix:
    wsl_kernel:
      - '6.6.75.2'    # WSL 2.5.1 - 2.5.7
      - '6.6.87.1'    # WSL 2.5.9
      - '6.6.87.2'    # WSL 2.5.10 - 2.6.3
      - '6.6.114.1'   # Future
      - '6.6.123.2'   # Latest kernel release
```

**S3 cache check:** Before building, check if the archive already exists in S3:
```bash
aws s3api head-object --bucket $S3_BUCKET --key $ARCHIVE 2>/dev/null
if [ $? -eq 0 ]; then
  echo "Archive $ARCHIVE already exists in S3, skipping build"
  exit 0
fi
```
To force a rebuild, delete the archive from S3: `aws s3 rm s3://$BUCKET/$ARCHIVE`

### Phase 6: ZFS version alignment across ecosystem

**Goal:** Unify ZFS version selection across all datadatdat repos.

**Current state (inconsistent):**
- `datadatdat-server` Dockerfile: `apt install zfsutils-linux` = 2.2.2 (Ubuntu 24.04 repos)
- `zfs-releases` build matrix: builds 2.2.8 and 2.3.4 for linuxkit/generic kernels
- `zfs-builder`: builds 2.3.4 (hardcoded in META)
- WSL2 module build: currently 2.2.2 to match server Dockerfile userland

**Target state:**
- Single ZFS version defined in one place (zfs-releases)
- All build pipelines consume this version
- Server Dockerfile installs userland from the S3 archive (not from apt)
- Kernel modules and userland always match

### Phase 7: Manual WSL2 version testing

**Goal:** Validate `d3 install` + `make e2e` across multiple WSL2 versions.

**Approach:** Serial testing on the Windows host. Each WSL version requires a different kernel, and only one WSL instance runs at a time.

**WSL version pinning:** WSL releases are available as MSI installers from GitHub:
```
https://github.com/microsoft/WSL/releases/download/<VERSION>/wsl.<VERSION>.0.x64.msi
```
Example: `wsl.2.5.7.0.x64.msi`

**Test procedure for each WSL version:**

```powershell
# 1. Stop Docker Desktop
Get-Process -Name "Docker*", "com.docker*" -ErrorAction SilentlyContinue | Stop-Process -Force

# 2. Shutdown WSL
wsl --shutdown

# 3. Install specific WSL version
# Download from: https://github.com/microsoft/WSL/releases/download/<VERSION>/wsl.<VERSION>.0.x64.msi
msiexec /i wsl.<VERSION>.0.x64.msi /quiet

# 4. Reset Ubuntu distro
wsl --unregister Ubuntu
wsl --install -d Ubuntu --no-launch

# 5. Verify kernel version
wsl -u root -e uname -r

# 6. Start Docker Desktop
& "C:\Program Files\Docker\Docker\Docker Desktop.exe"
# Wait for Docker to be ready
docker version

# 7. Run d3 install
./d3.exe install

# 8. Run E2E tests
make e2e
```

**Test matrix:**

| WSL Version | Kernel | ZFS Archive Expected | Test Status |
|------------|--------|---------------------|-------------|
| 2.5.7 | 6.6.75.x | `zfs-<VER>-modules-6.6.75.2-microsoft-standard-WSL2.tar.gz` | Pending |
| 2.5.9 | 6.6.87.1 | `zfs-<VER>-modules-6.6.87.1-microsoft-standard-WSL2.tar.gz` | Pending |
| 2.5.10 | 6.6.87.2 | `zfs-<VER>-modules-6.6.87.2-microsoft-standard-WSL2.tar.gz` | Pending |
| 2.6.1 | 6.6.87.x | Same as above | Pending |
| 2.6.3 | 6.6.87.2 | `zfs-<VER>-modules-6.6.87.2-microsoft-standard-WSL2.tar.gz` | **PASS** (validated) |

**Important:** After testing, restore the latest WSL version:
```powershell
wsl --update
```

### Phase 8: Automated WSL2 kernel detection

**Goal:** Auto-detect new WSL2 kernel releases and trigger module builds.

Add `wsl2-kernel-check.yml` workflow (similar to `ubuntu-kernel-check.yml`):
- Runs weekly on a schedule
- Checks `microsoft/WSL2-Linux-Kernel` for new releases
- If new kernel found and no S3 archive exists, triggers `wsl2-modules.yml`
- Creates a GitHub issue for tracking

### Phase 9: d3 install WSL2 detection

**Goal:** Detect WSL2 in the Go CLI and guide users.

In `d3 install`:
1. Detect if running on Windows with WSL2 backend
2. Check WSL2 kernel version via `wsl -e uname -r`
3. If kernel < 6.6.36.3: recommend `wsl --update`
4. If kernel >= 6.6.36.3: proceed normally (launch container handles ZFS loading)

### Phase 10: Cleanup

| Item | Action |
|------|--------|
| S3 bucket old precompiled module tarballs | Deprecate old format; new format includes userland |
| `datadatdat/zfs-builder` WSL path (`src/wsl.sh`) | Keep on experiment branch only |
| `datadatdat/docker-desktop-zfs-kernel` images | Deprecate |
| zfs-builder bzImage CI workflows | Keep on experiment branch only |
| 30-minute compile-from-source fallback | Removed from launch chain |
| Issue #79 | Close after Phase 7 testing complete |
