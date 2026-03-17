# WSL2 Prebuilt Kernel License Notice

The prebuilt WSL2 kernel images distributed in this repository's GitHub
Releases combine code from multiple sources under different open-source
licenses.

## Linux Kernel — GNU General Public License v2

The kernel itself (including the Microsoft WSL2 patches) is licensed under
the GNU General Public License, version 2.

- Source: https://github.com/microsoft/WSL2-Linux-Kernel
- License: [GPL-2.0](https://www.gnu.org/licenses/old-licenses/gpl-2.0.html)

## OpenZFS — Common Development and Distribution License v1.0

The ZFS filesystem code statically compiled into the kernel is licensed under
the Common Development and Distribution License, version 1.0.

- Source: https://github.com/openzfs/zfs
- License: [CDDL-1.0](https://opensource.org/licenses/CDDL-1.0)

## Build Infrastructure — Apache License 2.0

The build scripts and Docker infrastructure used to compile these kernels
are part of the datadatdat project.

- Source: https://github.com/datadatdat/zfs-builder
- License: [Apache-2.0](https://www.apache.org/licenses/LICENSE-2.0)

## Obtaining Source Code

To obtain the complete source code for any prebuilt kernel image:

1. **Linux Kernel**: Check the release notes for the exact Microsoft WSL2
   kernel tag used (e.g., `linux-msft-wsl-6.6.75.2`). Clone from:
   ```
   git clone --branch <tag> https://github.com/microsoft/WSL2-Linux-Kernel.git
   ```

2. **OpenZFS**: Check the release notes for the ZFS version (e.g., `2.3.4`).
   Clone from:
   ```
   git clone --branch zfs-<version> https://github.com/openzfs/zfs.git
   ```

3. **Build scripts**: The exact build process is documented in:
   ```
   git clone https://github.com/datadatdat/zfs-builder.git
   ```

## GPL/CDDL Compatibility Note

The combination of GPL-licensed kernel code with CDDL-licensed ZFS code is a
subject of ongoing legal interpretation. This distribution is provided as a
convenience for users who would otherwise compile the same combination
themselves. By downloading and using these images, you acknowledge that you
are choosing to combine these codebases on your own system.
