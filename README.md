# Datadatdat ZFS builds

[![Build Status](https://travis-ci.org/datadatdat/zfs-releases.svg?branch=master)](https://travis-ci.org/datadatdat/zfs-releases)
![GitHub last commit](https://img.shields.io/github/last-commit/datadatdat/zfs-releases)
![GitHub issues](https://img.shields.io/github/issues/datadatdat/zfs-releases)
![GitHub](https://img.shields.io/github/license/datadatdat/zfs-releases)

This repository uses the [zfs-builder](https://github.com/datadatdat/zfs-builder)
image to build a set of well-known Linux ZFS binaries, for kernel and userland,
suitable for use in the Datadatdat project.

The end result of this is a set of archives:

   * zfs-[zfs_version]-userland.tar.gz
   * zfs-[zfs_version]-[kernel_release].tar.gz

The build process is driven by the `build` script.

## Contributing

The ZFS builder project follows the Datadatdat community best practices:

  * [Contributing](https://github.com/datadatdat/.github/blob/master/CONTRIBUTING.md)
  * [Code of Conduct](https://github.com/datadatdat/.github/blob/master/CODE_OF_CONDUCT.md)
  * [Community Support](https://github.com/datadatdat/.github/blob/master/SUPPORT.md)

It is maintained by the [Datadatdat community maintainers](https://github.com/datadatdat/.github/blob/master/MAINTAINERS.md)

For more information on how it works, and how to build and release new versions,
see the [Development Guidelines](DEVELOPING.md).

## License

This is code is licensed under the Apache License 2.0. Full license is
available [here](./LICENSE).
