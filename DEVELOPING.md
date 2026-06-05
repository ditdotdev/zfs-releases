# ZFS Release Development

For general information about contributing changes, see the
[Contributor Guidelines](https://github.com/ditdotdev/.github/blob/master/CONTRIBUTING.md).

## How it works

The information about how to build these versions is driven by the `src/`
directory that has the following layout:

   src/
      [kernel_release]/
         uname
         [config.gz | centos-release] (optional)

The kernel release should be the result of "uname -r", and the uname file should
contain the output of "uname -a". The system will mount the directory at
`/config`, so any variant-specific file contents (`centos-release`, `config.gz`,
etc) should be placed there.

The results are placed in the 'out/archive' directory. The script must be
invoked by a user with sufficient privileges to run docker containers.

## Building

The `build` script will iterate over the contents of the directory and invoke
the zfs-builder container to perform the build.

```
./build [-u] [-k] [-z zfs_version] [kernel_release ...]
```

The '-u' option will only build userland, while '-k' will only build the kernel.
By default, it will build both for the first found kernel release, and then
kernel only for the remainder. The '-z' option will specify the ZFS version(s)
to build.

When you add a new kernel version, add the appropriate entry to `src/` and
update the matrix in `.github/workflows/push.yml` if you want it built on every push.

## Testing

You should be able to run the builds for any platforms that you are changing.
If you change the build script itself, be sure to build all of the known
platforms (this can take a while).

## Release

We build releases using GitHub Actions with three workflows:

1. **push.yml** - Production builds for ZFS 2.3.4 and 2.2.8 on every push
   - Multi-architecture support (amd64 and arm64)
   - Builds both userland and kernel modules for specific kernel versions
   - Uses Docker Buildx with QEMU emulation for cross-platform builds

2. **nightly.yml** - Nightly kernel compatibility checks for ZFS 2.1.5
   - Runs at 5:00 AM UTC daily
   - Tests against available Ubuntu kernels
   - Helps detect kernel compatibility issues early

3. **manual.yml** - Manual kernel builds for testing
   - Triggered via workflow_dispatch
   - Allows testing specific ZFS version + kernel combinations

All workflows use the [zfs-builder](https://github.com/ditdotdev/zfs-builder) 
Docker image, which provides a consistent build environment with all required 
dependencies.

Once a particular set of binaries is built, it shouldn't ever need to be updated.
Artifacts are published to an S3 bucket.

If we do need to re-build a particular release, we can remove it manually via
the S3 console, or trigger a manual workflow run.
