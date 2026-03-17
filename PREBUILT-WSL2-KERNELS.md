# Plan: Prebuilt WSL2 Kernels with ZFS for Windows Users

## Context

Windows users of datadatdat need a custom WSL2 kernel with ZFS statically compiled in. The default WSL2 kernel ships with `CONFIG_MODULES=n`, which means **kernel modules cannot be loaded at all** — the existing zfs-releases approach of building `.ko` module files does not work for WSL2. The only viable delivery mechanism is a complete prebuilt `bzImage` with ZFS built-in.

Currently users must compile this kernel themselves — a 30+ minute process requiring build dependencies and Linux expertise. We want to provide prebuilt `bzImage` files via GitHub Releases so users can simply download, update `.wslconfig`, and `wsl --shutdown`.

The infrastructure is ~80% ready: `zfs-builder/src/wsl.sh` already builds WSL2 kernels with ZFS statically compiled, and `zfs-releases` has GitHub Actions workflows for building ZFS artifacts. The gaps are: WSL2 kernels aren't in the build matrix, there's no GitHub Releases publishing, and `wsl.sh` has CI-incompatible assumptions (reads `/proc/config.gz`, checks running kernel).

## Testing Strategy

### Why Standard CI Can't Fully Validate WSL2 Kernels

- **`ubuntu-latest` runners** can build the kernel but can't test it as a WSL2 kernel (no Windows/Hyper-V)
- **`windows-latest` runners are ephemeral** — can't persist `.wslconfig`, and WSL2 custom kernel setup requires a reboot cycle
- **WSL2's `CONFIG_MODULES=n`** means we can't use the module-loading approach — must validate the full kernel image boots and has ZFS

### Approach: Build in CI + Vagrant VM Validation Locally

**Tier 1: Automated build validation (CI, every workflow run)**
- Compile the kernel on GitHub Actions Ubuntu runner
- Verify bzImage is produced and is a valid x86 kernel image (`file bzImage`)
- Verify file size is in expected range (10-20MB)
- Build all active WSL2 kernel versions × ZFS versions matrix
- Publish as **draft** GitHub Release

**Tier 2: Vagrant + Hyper-V full E2E validation (local, REQUIRED before publishing any release)**

Every prebuilt kernel MUST pass the full BATS E2E test suite before release. Run `test-wsl2-kernel.ps1` on your Windows machine:

1. Spins up a Windows 11 Vagrant box using the Hyper-V provider
2. Provisions WSL2 + Docker Desktop + Go + BATS + dev tools inside the VM
3. Copies the bzImage into the VM and configures `.wslconfig`
4. Restarts WSL2 inside the VM
5. Builds d3 CLI (`make build`) inside WSL2
6. Runs `setup-zfs-pools.sh` to create ZFS pools
7. **Runs `make e2e`** — full E2E suite: install, getting-started, tags, docker-context, container-lifecycle, data-import, S3 workflow, SSH workflow, upgrade, uninstall
8. **Runs `make e2e-server`** — server workflows: auth, org, billing, clone-commit, fork, push-pull-tags
9. Reports per-suite pass/fail results and destroys the VM
10. **ALL BATS tests must pass** before promoting draft → published release

**Why Vagrant + Hyper-V:**
- Hyper-V is already active on your machine (for Docker/WSL2) — no hypervisor conflict
- VirtualBox would require disabling Hyper-V (`virtualbox_switch.ps1`), breaking Docker/WSL2
- Vagrant provides declarative, reproducible VM definitions via Vagrantfile
- Windows Vagrant boxes exist (e.g., `gusztavvargadr/windows-11`)
- Nested virtualization: Hyper-V supports `Set-VMProcessor -ExposeVirtualizationExtensions $true` so WSL2 works inside the VM

**Vagrant test infrastructure:**
```
datadatdat/tests/wsl2-kernel/
├── Vagrantfile              # Windows 11 box, Hyper-V provider, nested virt
├── provision-wsl2.ps1       # Install WSL2 + Ubuntu + Docker Desktop + Go + BATS
├── deploy-kernel.ps1        # Copy bzImage, configure .wslconfig, restart WSL2
├── run-e2e.sh               # Run inside WSL2: kernel checks, make e2e, make e2e-server
├── test-wsl2-kernel.ps1     # Orchestrator: vagrant up → provision → test → destroy
└── README.md                # Usage instructions
```

**Usage:**
```powershell
# Test a specific bzImage before promoting the draft release
cd datadatdat\tests\wsl2-kernel
.\test-wsl2-kernel.ps1 -BzImagePath C:\path\to\bzImage

# Test directly from a draft GitHub Release
.\test-wsl2-kernel.ps1 -ReleaseTag wsl2-kernel-6.6.75.2-zfs-2.3.4
```

**E2E validation checks (`run-e2e.sh`):**
1. Kernel validation: `uname -r` matches expected version, `grep zfs /proc/filesystems`
2. ZFS pool setup: `setup-zfs-pools.sh` creates datadatdat pools
3. `make e2e` — 12 BATS test suites covering the full CLI + container lifecycle
4. `make e2e-server` — 10+ BATS test suites covering server remote workflows

**No kernel release is published unless ALL BATS tests pass.** This ensures datadatdat works 100% on each kernel.

## Bug Fix: Launcher Doesn't Detect Built-in ZFS (datadatdat-server repo)

### The Problem

When a WSL2 user has ZFS statically compiled into the kernel (CONFIG_MODULES=n), the launcher container unnecessarily falls through the entire module-loading chain and pulls `datadatdat/zfs-builder:latest`.

### Observed Output

**`d3 install` output:**
```
$ ./d3 install
Initializing datadatdat infrastructure
Checking docker installation
Latest docker image downloaded
Datadatdat CLI successfully installed, happy data versioning :)
Checking if compatible ZFS is running
Checking if compatible system ZFS is available
Checking if compatible compiled ZFS is available
Checking if precompiled ZFS is available for '5.15.167.4-microsoft-standard-WSL2-titan-zfs'
Building ZFS kernel modules (this could take 30 minutes, submit a request for 5.15.167.4-microsoft-standard-WSL2-titan-zfs prebuilt binaries)
```

**`docker logs datadatdat-docker-launch` (key excerpts):**
```
KERNEL_RELEASE = 5.15.167.4-microsoft-standard-WSL2-titan-zfs

DATADATDAT START Checking if compatible ZFS is running
ZFS is not currently loaded                              ← is_zfs_loaded() misses built-in ZFS
DATADATDAT END

DATADATDAT START Checking if compatible system ZFS is available
No ZFS module found
DATADATDAT END

DATADATDAT START Checking if compatible compiled ZFS is available
No ZFS module found
DATADATDAT END

DATADATDAT START Checking if precompiled ZFS is available for '5.15.167.4-microsoft-standard-WSL2-titan-zfs'
curl: (22) The requested URL returned error: 404   ← custom -titan-zfs suffix not in S3
No ZFS module found
DATADATDAT END

DATADATDAT START Building ZFS kernel modules (this could take 30 minutes...)
WARNING: The requested image's platform (linux/arm64) does not match the detected host platform (linux/amd64/v3)
                                                    ← Bug 2: wrong arch zfs-builder image!

# Inside zfs-builder container:
Starting ZFS availability check...
✓ ZFS kernel support detected                       ← zfs-builder's build.sh DOES check /proc/filesystems
✗ ZFS device node (/dev/zfs) not found
✗ ZFS userspace tools not found
⚠️  Incomplete ZFS support detected. Kernel build required.  ← but fails because /dev/zfs + zpool missing in container
ZFS not fully available - proceeding with kernel build for type: wsl

# wsl.sh's check_zfs_availability() also detects built-in:
✓ ZFS kernel support detected in /proc/filesystems
🎉 ZFS is built into the kernel! No kernel build needed.
ZFS kernel support already available - skipping kernel build
DATADATDAT END

ZFS is built into the kernel                        ← load_zfs_module() finally detects it
DATADATDAT START Creating shared mounts
DATADATDAT END
DATADATDAT FINISHED                                 ← launcher succeeds (eventually)
```

**`docker logs datadatdat-docker-server` (key excerpts):**
```
Detected PostgreSQL version: 16
Mounting datadatdat-docker/db at /var/lib/datadatdat-docker/mnt/_db
mount -i -t zfs datadatdat-docker/db ...            ← ZFS mount works fine
Starting postgres
Postgres started
Application started in 1.417 seconds.
Responding at http://0.0.0.0:5001                   ← server running successfully
ServiceLocator: Available providers: datadatdat, nop, ssh, s3, s3web
```

### Analysis

**The system eventually works** — but with unnecessary overhead:

1. **Bug 1: `is_zfs_loaded()` misses built-in ZFS** — falls through all 5 steps of the fallback chain
2. **Bug 2: Wrong-arch zfs-builder** — `datadatdat/zfs-builder:latest` resolves to `linux/arm64` on an `amd64/v3` host
3. **Bug 3: zfs-builder's `check_zfs_availability()` is too strict** — it detects ZFS in `/proc/filesystems` but still fails because `/dev/zfs` and `zpool` don't exist inside the container. It requires ALL three (kernel + device + userspace) to consider ZFS "available", when kernel support alone should be sufficient for skipping the build
4. The launcher's `load_zfs_module()` (line 117-125) does eventually detect built-in ZFS via `/proc/filesystems` and succeeds
5. The server container mounts ZFS datasets, starts PostgreSQL, and runs normally

**Impact:** On a clean install, the user sees "Building ZFS kernel modules (this could take 30 minutes...)" which is alarming, pulls an unnecessary Docker image (wrong arch at that), runs it only to have it discover ZFS is already there, then continues normally. With the fix, step 1 succeeds immediately and the whole process takes <2 seconds instead of ~10+ seconds.

Note: The kernel release string `5.15.167.4-microsoft-standard-WSL2-titan-zfs` includes the custom `-titan-zfs` suffix from the build, so S3 precompiled lookup at step 4 can never match (S3 only has standard kernel releases).

**Root cause:** `is_zfs_loaded()` in `zfs.sh:88-90` only checks `lsmod`, which doesn't show built-in kernel features:

```bash
function is_zfs_loaded() {
  lsmod | grep "^zfs " >/dev/null 2>&1   # ← misses built-in ZFS
}
```

**The chain of failure** (launch script lines 109-115):
1. `check_running_zfs()` → `is_zfs_loaded()` → `lsmod` → **false** (ZFS is built-in, not a module)
2. `load_zfs` system modules → no `.ko` files → **fails**
3. `load_zfs` compiled modules → none cached → **fails**
4. `load_precompiled_zfs` → downloads from S3, tries `modprobe` → **fails** (CONFIG_MODULES=n)
5. `compile_and_load_zfs` → **pulls `datadatdat/zfs-builder:latest`** → builds for 30 min → **fails**

Note: `load_zfs_module()` at line 117-125 **does** check `/proc/filesystems` for built-in ZFS, but it's only called deep in steps 2-5 after version checks have already failed.

### The Fix

**File:** `datadatdat-server/server/src/scripts/zfs.sh`

Update `is_zfs_loaded()` to also check for built-in ZFS:

```bash
function is_zfs_loaded() {
  # Check for ZFS as a loadable module
  lsmod | grep "^zfs " >/dev/null 2>&1 && return 0
  # Check for ZFS built into the kernel (e.g., custom WSL2 kernel)
  grep -q "^nodev.*zfs" /proc/filesystems 2>/dev/null && return 0
  return 1
}
```

And update `get_running_zfs_version()` to handle built-in ZFS where `/sys/module/zfs/version` may not exist:

```bash
function get_running_zfs_version() {
  # Module version file (standard modules)
  cat /sys/module/zfs/version 2>/dev/null && return 0
  # For built-in ZFS, try zfs version command
  zfs version 2>/dev/null | grep -oP '[\d.]+' | head -1
}
```

This makes `check_running_zfs()` succeed immediately at step 1, skipping the entire fallback chain.

## Implementation

### Phase 0: Fix built-in ZFS detection (datadatdat-server + zfs-builder repos)

**Bug 1 fix — `datadatdat-server/server/src/scripts/zfs.sh`:**
1. Update `is_zfs_loaded()` (line 88-90) to check `/proc/filesystems` for built-in ZFS
2. Update `get_running_zfs_version()` (line 97-99) to handle built-in ZFS (no `/sys/module/zfs/version`)
3. This makes `check_running_zfs()` succeed at step 1, skipping the entire fallback chain

**Bug 2 fix — zfs-builder image architecture:**
- Investigate why `datadatdat/zfs-builder:latest` resolves to `linux/arm64` on an `amd64` host
- Likely needs a multi-arch manifest or explicit `--platform linux/amd64` in the build/push workflow

**Bug 3 fix — `zfs-builder/src/build.sh`:**
- `check_zfs_availability()` (lines 11-49) requires all three: kernel + `/dev/zfs` + `zpool`
- Inside a container, `/dev/zfs` and `zpool` won't exist even when ZFS is built into the host kernel
- Fix: if `/proc/filesystems` shows ZFS, that's sufficient to skip the build — don't require device node or userspace tools

### Phase 1: Make `wsl.sh` CI-compatible (zfs-builder repo)

**File:** `zfs-builder/src/wsl.sh`

1. **`get_kernel_src()` (line 52)** — Accept explicit `WSL2_KERNEL_TAG` env var. When set, use it directly instead of querying GitHub API to match `uname -r`. CI specifies the exact tag (e.g., `linux-msft-wsl-6.6.75.2`).

2. **`prepare_kernel()` (lines 92-113)** — Remove dependency on `/proc/config.gz`. The Microsoft kernel tree includes `Microsoft/config-wsl` which is the correct config. `build_kernel()` already uses it at line 173 (`KCONFIG_CONFIG="Microsoft/config-wsl"`), making `prepare_kernel()` redundant. Remove `prepare_kernel()` entirely — `build_kernel()` already does `olddefconfig` + `prepare` + `scripts`.

3. **`build()` (lines 262-269)** — Skip `check_zfs_availability()` when `CI_BUILD=1` env var is set. In CI, ZFS will never be pre-available, and we always want to build.

4. **`build.sh` (lines 88-96)** — Same `CI_BUILD=1` bypass for the outer dispatcher's `check_zfs_availability()`.

5. **`get_zfs_builtin()` (line 123)** — Update default from `zfs-2.1.5` to `zfs-2.3.4`.

### Phase 2: Add WSL2 build workflow (zfs-releases repo)

**New file:** `zfs-releases/.github/workflows/wsl2.yml`

Separate workflow because:
- Produces `bzImage`, not kernel modules — fundamentally different artifact
- Needs `permissions: contents: write` for GitHub Releases (existing `push.yml` has `contents: read`)
- 30+ minute builds shouldn't slow down module builds

```yaml
name: WSL2 Kernel Build
on:
  workflow_dispatch:
    inputs:
      wsl_tag:
        description: 'WSL2 kernel tag (e.g., linux-msft-wsl-6.6.75.2)'
      zfs_version:
        description: 'ZFS version (e.g., 2.3.4)'
  schedule:
    - cron: '0 6 * * 1'  # Weekly Monday

permissions:
  contents: write

jobs:
  build_image:
    # Same pattern as push.yml — build zfs-builder Docker image (amd64 only)

  build_wsl2_kernel:
    needs: build_image
    runs-on: ubuntu-latest
    strategy:
      fail-fast: false
      matrix:
        include:
          # All active Microsoft WSL2 kernel releases
          # Populate from https://github.com/microsoft/WSL2-Linux-Kernel/releases
          - wsl_tag: linux-msft-wsl-6.6.75.2
            kernel_release: 6.6.75.2-microsoft-standard-WSL2
            zfs: 2.3.4
          # ... additional active releases
    steps:
      - Download zfs-builder image artifact
      - Load Docker image
      - docker run with KERNEL_RELEASE, ZFS_VERSION, WSL2_KERNEL_TAG, CI_BUILD=1
      - Extract bzImage from container /out/
      - Validate: `file bzImage` shows Linux kernel x86 boot executable
      - Validate: file size 10-20MB
      - Upload bzImage as workflow artifact

  publish_draft_release:
    needs: build_wsl2_kernel
    steps:
      - Download all bzImage artifacts
      - Generate sha256 checksums
      - Create/update draft GitHub Release
      - Tag format: wsl2-kernel-{kernel_version}-zfs-{zfs_version}
      - Attach: bzImage, CHECKSUMS.sha256, INSTALL.md, WSL2-LICENSE.md
```

Note: trigger is `workflow_dispatch` + weekly schedule, NOT `on: push`. WSL2 kernel builds are expensive (30+ min each × matrix size) and don't need to run on every commit.

**Skip already-built kernels:** Before building, check if a GitHub Release already exists for this kernel+ZFS combination:
```bash
# Check if release already exists — skip build if so
gh release view "wsl2-kernel-${kernel_release}-zfs-${zfs_version}" --repo datadatdat/zfs-releases > /dev/null 2>&1
if [ $? -eq 0 ]; then
  echo "Release already exists for ${kernel_release} + ZFS ${zfs_version}, skipping"
  exit 0
fi
```
This follows the same pattern as the existing `build` script's `archive_exists()` S3 check (line 30-32 of `zfs-releases/build`). Prevents wasting workflow minutes rebuilding kernels that are already published.

### Phase 3: WSL2 kernel version discovery (zfs-releases repo)

**New file:** `zfs-releases/.github/workflows/wsl2-kernel-check.yml`

Daily check workflow (similar to existing `ubuntu-kernel-check.yml`) that monitors Microsoft's WSL2 kernel releases and alerts when new versions are available:

**Schedule:** Daily at 6 AM UTC

**Steps:**
1. Query `https://api.github.com/repos/microsoft/WSL2-Linux-Kernel/releases` for all releases
2. Extract release tags (e.g., `linux-msft-wsl-6.6.75.2`) and derive kernel release strings
3. Read the current build matrix from `wsl2.yml` to get the list of already-supported versions
4. Compare: identify any new releases not already in the matrix
5. If new version(s) found → create a GitHub issue with:
   - New kernel tag name and version
   - Link to Microsoft's release notes
   - Snippet to add to the `wsl2.yml` matrix
   - Checklist: add to matrix → build → Vagrant E2E test → publish

**Tracking known versions:** Store the list of already-processed kernel versions in a file (e.g., `zfs-releases/wsl2-known-versions.txt`) that the workflow updates. This avoids creating duplicate issues for the same release.

**New kernel lifecycle:**
1. `wsl2-kernel-check.yml` detects new release → creates issue
2. Maintainer adds new version to `wsl2.yml` matrix
3. Trigger `wsl2.yml` via `workflow_dispatch` → builds bzImage
4. Run `test-wsl2-kernel.ps1` → `make e2e` + `make e2e-server` → all BATS pass
5. Promote draft → published release
6. Update `wsl2-known-versions.txt`

**Also monitors:** OpenZFS releases at `https://api.github.com/repos/openzfs/zfs/releases` — new ZFS versions also require rebuilding all kernels in the matrix.

### Phase 4: PowerShell installer script (datadatdat repo)

**New file:** `datadatdat/scripts/Install-D3Kernel.ps1`

```powershell
# Usage:
#   .\Install-D3Kernel.ps1              # Download and install latest
#   .\Install-D3Kernel.ps1 -CheckOnly   # Just check for updates
#   .\Install-D3Kernel.ps1 -ZfsVersion 2.3.4

# Steps:
# 1. Detect current WSL2 kernel version (wsl uname -r)
# 2. Query GitHub Releases API for matching published wsl2-kernel-* release
# 3. Download bzImage to $env:USERPROFILE\.datadatdat\kernels\
# 4. Verify sha256 checksum
# 5. Back up existing .wslconfig (if exists)
# 6. Update/create .wslconfig with kernel= path (double-backslash escaping)
# 7. Print: "Run 'wsl --shutdown' then restart your WSL2 terminal"
```

### Phase 5: Update documentation (datadatdat repo)

**Modified:** `datadatdat/wsl-kernel-zfs.md`
- Add "Quick Start (Recommended)" section at top:
  - Download prebuilt kernel from GitHub Releases
  - Or run `Install-D3Kernel.ps1`
  - Update `.wslconfig`, `wsl --shutdown`
- Rename existing content to "Building from Source" section
- Add note explaining why custom kernel is required (CONFIG_MODULES=n)

**Modified:** `datadatdat/cleanslate/README.md`
- Reference prebuilt kernel download in prerequisites
- Link to `Install-D3Kernel.ps1`

### Phase 7: Vagrant + Hyper-V test infrastructure (datadatdat repo)

**New directory:** `datadatdat/tests/wsl2-kernel/`

**Vagrantfile:**
- Box: `gusztavvargadr/windows-11` (or similar Windows 11 evaluation box)
- Provider: Hyper-V with nested virtualization enabled
- Memory: 4GB+ (WSL2 needs headroom)
- Synced folder: share the bzImage into the VM

**provision-wsl2.ps1** (runs inside the VM):
- Enable Windows features: `Microsoft-Windows-Subsystem-Linux`, `VirtualMachinePlatform`
- Reboot VM
- `wsl --install -d Ubuntu` (or `wsl --install --no-distribution` + manual import)
- Wait for WSL2 to be ready

**deploy-kernel.ps1** (runs inside the VM):
- Copy bzImage to `C:\Users\vagrant\.datadatdat\kernels\`
- Create/update `.wslconfig` with `kernel=` path
- `wsl --shutdown`

**run-e2e.sh** (runs inside WSL2 in the VM):
- Verify `uname -r` matches expected kernel
- Verify `grep zfs /proc/filesystems`
- Clone datadatdat repo, build d3 CLI (`make build`)
- Run `setup-zfs-pools.sh` to create ZFS pools
- Run `make e2e` — full E2E suite: install, getting-started, tags, docker-context, container-lifecycle, data-import, S3 workflow, SSH workflow, upgrade, uninstall
- Run `make e2e-server` (if datadatdat-server is available) — server workflows: auth, org, billing, clone-commit, fork, push-pull-tags
- Exit 0 on all pass, non-zero on failure

**test-wsl2-kernel.ps1** (orchestrator, runs on host):
```powershell
param(
    [string]$BzImagePath,      # Local path to bzImage
    [string]$ReleaseTag,       # Or download from draft release
    [switch]$KeepVM,           # Don't destroy VM after test (for debugging)
    [switch]$SkipServerTests   # Skip make e2e-server (no server available)
)
# 1. Download bzImage from GitHub Release if -ReleaseTag specified
# 2. vagrant up (provisions Windows 11 + WSL2 + Docker Desktop + dev tools)
# 3. Copy bzImage into VM, configure .wslconfig
# 4. Restart WSL2 in the VM
# 5. Run run-e2e.sh inside WSL2 (make e2e + optionally make e2e-server)
# 6. Report results (which test suites passed/failed)
# 7. vagrant destroy (unless -KeepVM)
```

**VM provisioning requirements:**
- Docker Desktop (needed for container lifecycle, ZFS volume operations)
- Go 1.25.1+ (to build d3 CLI)
- BATS (npm install -g bats) — test framework
- AWS credentials (for S3 workflow tests) — passed via env vars or Vagrant secrets
- SSH keys (for SSH workflow tests) — generated during provisioning
- Git + Make + standard build tools
- The datadatdat repo cloned inside WSL2

**Prerequisites on host:** Vagrant installed, Hyper-V enabled (already active)

### Phase 6: Licensing (zfs-releases repo)

**New file:** `zfs-releases/WSL2-LICENSE.md`
- GPL v2 (Linux kernel) + CDDL (OpenZFS) notice
- Links to source repos: microsoft/WSL2-Linux-Kernel, openzfs/zfs, datadatdat/zfs-builder
- Included as a release asset with each GitHub Release

## Critical Files

| File | Repo | Action |
|------|------|--------|
| `server/src/scripts/zfs.sh` | datadatdat-server | Fix: `is_zfs_loaded()` + `get_running_zfs_version()` for built-in ZFS |
| `src/wsl.sh` | zfs-builder | Modify: CI env vars, remove `prepare_kernel()`, update ZFS default |
| `src/build.sh` | zfs-builder | Modify: `CI_BUILD=1` bypass for `check_zfs_availability()` |
| `.github/workflows/wsl2.yml` | zfs-releases | New: WSL2 build + draft release workflow |
| `.github/workflows/wsl2-kernel-check.yml` | zfs-releases | New: daily version monitoring |
| `tests/wsl2-kernel/Vagrantfile` | datadatdat | New: Windows 11 Hyper-V VM definition |
| `tests/wsl2-kernel/test-wsl2-kernel.ps1` | datadatdat | New: orchestrator script |
| `tests/wsl2-kernel/provision-wsl2.ps1` | datadatdat | New: WSL2 installation in VM |
| `tests/wsl2-kernel/deploy-kernel.ps1` | datadatdat | New: bzImage deploy + .wslconfig |
| `tests/wsl2-kernel/smoke-test.sh` | datadatdat | New: ZFS validation inside WSL2 |
| `scripts/Install-D3Kernel.ps1` | datadatdat | New: PowerShell installer for users |
| `wsl-kernel-zfs.md` | datadatdat | Update: add prebuilt quick start, explain CONFIG_MODULES=n |
| `cleanslate/README.md` | datadatdat | Update: reference prebuilt kernels |
| `WSL2-LICENSE.md` | zfs-releases | New: composite license |

## Verification

### Automated (CI)
- Kernel compilation succeeds for all matrix entries
- `file bzImage` confirms valid Linux x86 boot executable
- File size in expected range (10-20MB)
- Draft GitHub Release created with all artifacts + checksums

### Vagrant VM E2E test (before publishing)
```powershell
cd datadatdat\tests\wsl2-kernel
.\test-wsl2-kernel.ps1 -ReleaseTag wsl2-kernel-6.6.75.2-zfs-2.3.4
```
This spins up a fresh Windows 11 Hyper-V VM, provisions WSL2 + Docker Desktop + Go + BATS, deploys the bzImage, runs `make e2e` (install, getting-started, tags, docker-context, container-lifecycle, data-import, S3 workflow, SSH workflow, upgrade, uninstall) and optionally `make e2e-server` (auth, org, billing, clone-commit, fork workflows). Reports per-suite pass/fail results and destroys the VM. On all-pass → promote draft release to published.

### Installer validation
1. Run `Install-D3Kernel.ps1 -CheckOnly` → shows available update
2. Run `Install-D3Kernel.ps1` → downloads, configures, prompts for restart
3. After restart → verify kernel + ZFS as above

## Development Workflow

### Branch Strategy
All work is done on feature branches. PRs are only created after all local testing passes.

| Repo | Branch Name | PR When |
|------|------------|---------|
| datadatdat-server | `fix/builtin-zfs-detection` | After Phase 0 tests pass |
| zfs-builder | `fix/ci-compatibility` | After Phase 0+1 tests pass |
| zfs-releases | `feature/wsl2-kernel-builds` | After Phase 2+3+6 workflow tested |
| datadatdat | `feature/wsl2-prebuilt-kernels` | After Phase 4+5+7 + full E2E pass |

### Commit Discipline
- **Run all linters before every commit**: `gofmt -s -w .` for Go repos, `./gradlew ktlint` for Kotlin repos, `shellcheck` for shell scripts
- Commit frequently to branches — small, logical commits with clear messages
- PRs only after all local testing is successful

### Test-Driven Development (TDD)

**Bug fixes (Phase 0) — write failing tests FIRST, then fix:**

1. **Bug 1: `is_zfs_loaded()` misses built-in ZFS**
   - New test: `datadatdat-server/server/src/scripts/tests/test_zfs.bats`
   - Test case: Mock `/proc/filesystems` with ZFS entry, no `lsmod` output → assert `is_zfs_loaded` returns 0
   - Test case: No ZFS in `/proc/filesystems`, no `lsmod` output → assert `is_zfs_loaded` returns 1
   - Test case: ZFS in `lsmod` → assert `is_zfs_loaded` returns 0 (existing behavior preserved)

2. **Bug 2: Wrong-arch zfs-builder image**
   - Test: CI workflow step that verifies `docker inspect datadatdat/zfs-builder:latest --format '{{.Architecture}}'` matches runner arch

3. **Bug 3: `check_zfs_availability()` too strict in container**
   - New test: `zfs-builder/src/tests/test_build.bats`
   - Test case: `/proc/filesystems` has ZFS, no `/dev/zfs`, no `zpool` → assert build is skipped
   - Test case: No ZFS anywhere → assert build proceeds

**New functionality — write tests alongside code:**

4. **`wsl.sh` CI compatibility (Phase 1)**
   - Test: `WSL2_KERNEL_TAG` env var is respected in `get_kernel_src()`
   - Test: `CI_BUILD=1` skips `check_zfs_availability()` in `build()`
   - Test: `prepare_kernel()` removal doesn't break `build_kernel()` flow

5. **Install-D3Kernel.ps1 (Phase 4)**
   - Pester tests: checksum verification, `.wslconfig` generation, backup behavior

6. **Vagrant test infrastructure (Phase 7)**
   - The Vagrant E2E test IS the test — `make e2e` + `make e2e-server` validates everything

### Test Execution Before PR

Before creating any PR, run:
```bash
# datadatdat-server
cd datadatdat-server && bats server/src/scripts/tests/

# zfs-builder
cd zfs-builder && bats src/tests/

# datadatdat (full E2E on the branch)
cd datadatdat && make e2e && make e2e-server
```

## Repos Involved

| Repo | Phases | Changes |
|------|--------|---------|
| **datadatdat-server** | 0 | Fix `zfs.sh` built-in ZFS detection |
| **zfs-builder** | 0, 1 | Fix `build.sh` + arch issue; make `wsl.sh` CI-compatible |
| **zfs-releases** | 2, 3, 6 | New workflows (`wsl2.yml`, `wsl2-kernel-check.yml`), licensing |
| **datadatdat** | 4, 5, 7 | Installer script, docs, Vagrant test infrastructure |

## Execution Order

0. **Persist this plan** as `PREBUILT-WSL2-KERNELS.md` in the root of zfs-releases repo
1. **Phase 0** (fix built-in ZFS detection in launcher) — immediate bug fix, independent of other phases
2. **Phase 1** (wsl.sh CI fixes) — must come before Phase 2
3. **Phase 2** (wsl2.yml workflow) — depends on Phase 1
4. **Phase 6** (licensing) — parallel with Phase 2
5. **Phase 3** (version monitoring) — parallel with Phase 2
6. **Phase 7** (Vagrant test infra) — parallel with Phase 2, needed before first release
7. **Phase 4** (installer) — depends on Phase 2 (needs releases to exist)
8. **Phase 5** (docs) — last, after everything works
