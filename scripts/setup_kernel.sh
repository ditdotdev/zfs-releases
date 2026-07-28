#!/bin/sh
# Copyright Dit 2026
# SPDX-License-Identifier: BUSL-1.1


set -e

mkdir -p src/$(uname -r)
printf %s "$(uname -a)" >> src/$(uname -r)/uname
