#!/bin/bash -ex

# Copyright 2016 go-dockerclient authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

if [[ $TRAVIS_GO_VERSION == 1.10* ]]; then
	export SKIP_FMT_CHECK=1
fi

make test

if [[ $TRAVIS_OS_NAME == "linux" ]]; then
	make integration
fi
