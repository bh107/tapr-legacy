#!/bin/sh

set -eu

echo '-X "github.com/bh107/tapr/build.tag='$(git rev-parse --short HEAD)'"' \
     '-X "github.com/bh107/tapr/build.time='$(date -u '+%Y/%m/%d %H:%M:%S')'"'
