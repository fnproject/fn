#!/bin/bash

license() {
  apache=$(grep -rs "Apache License, Version 2.0" vendor/$1)
  if [[ ! -z $apache ]]; then
    echo "$1 Apache 2.0"
    return
  fi

  bsd_file=$(grep -lrs "Redistribution and use in source and binary forms, with or without" vendor/$1 | head -n 1)
  if [[ ! -z $bsd_file ]]; then
    bsd3=$(grep "this software without specific prior written permission" $bsd_file)
    if [[ ! -z $bsd3 ]]; then
      echo "$1 BSD 3"
    else
      echo "$1 BSD 2"
    fi
    return
  fi

  mit=$(grep -rs "Permission is hereby granted, free of charge" vendor/$1)
  if [[ ! -z $mit ]]; then
    echo "$1 MIT"
    return
  fi

  moz=$(grep -rs "This Source Code Form is subject to the terms of the Mozilla" vendor/$1)
  if [[ ! -z $moz ]]; then
    echo "$1 Mozilla Public License 2.0"
  fi

  # TODO others

  echo "$1 No License Found"
}

deps=$(cat glide.lock | grep -E '\- name:' | awk '{print $3}')

for dep in $deps; do
  license $dep
done
