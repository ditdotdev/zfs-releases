# Dit ZFS builds

![GitHub last commit](https://img.shields.io/github/last-commit/ditdotdev/zfs-releases)
![GitHub issues](https://img.shields.io/github/issues/ditdotdev/zfs-releases)
![GitHub](https://img.shields.io/github/license/ditdotdev/zfs-releases)

This repository uses the [zfs-builder](https://github.com/ditdotdev/zfs-builder)
image to build a set of well-known Linux ZFS binaries, for kernel and userland,
suitable for use in the Dit project.

The end result of this is a set of archives:

   * zfs-[zfs_version]-userland.tar.gz
   * zfs-[zfs_version]-[kernel_release].tar.gz

The build process is driven by the `build` script.

## Contributing

The ZFS builder project follows the Dit community best practices:

  * [Contributing](https://github.com/ditdotdev/.github/blob/master/CONTRIBUTING.md)
  * [Code of Conduct](https://github.com/ditdotdev/.github/blob/master/CODE_OF_CONDUCT.md)
  * [Community Support](https://github.com/ditdotdev/.github/blob/master/SUPPORT.md)

It is maintained by the [Dit community maintainers](https://github.com/ditdotdev/.github/blob/master/MAINTAINERS.md)

For more information on how it works, and how to build and release new versions,
see the [Development Guidelines](DEVELOPING.md).
