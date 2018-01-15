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

  moz=$(grep -rs "Mozilla Public License" vendor/$1)
  if [[ ! -z $moz ]]; then
    echo "$1 Mozilla Public License 2.0"
    return
  fi

  isc=$(grep -rs "Permission to use, copy, modify, and distribute this software for any" vendor/$1)
  if [[ ! -z $isc ]]; then
    echo "$1 ISC License"
    return
  fi

  unlicense=$(grep -rs "This is free and unencumbered software released into the public domain." vendor/$1)
  if [[ ! -z $unlicense ]]; then
    echo "$1 Unlicense"
    return
  fi

  cc=$(grep -rs "creativecommons" vendor/$1)
  if [[ ! -z $cc ]]; then
    echo "$1 Creative Commons"
    return
  fi

  # TODO others

  echo "$1 No License Found"
}

deps=$(cat Gopkg.lock | grep -E 'name.*=.*"' | awk '{print $3}' | tr -d '"')

for dep in $deps; do
  license $dep
done
