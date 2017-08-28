#!/bin/bash

set -e

`dirname $0`/build.sh

$GOPATH/bin/whatevrme_site $@

