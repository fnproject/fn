Changelog
=========

## head
*   Fix leak on sender create with unresolvable destination (GH-34).

## v3.1.0 2016-05-30
*   `NewClientWithSender(Sender, string) (Statter, error)` method added to
    enable building a Client from a prefix and an already created Sender.
*   Add stat recording sender in submodule statsdtest (GH-32).
*   Add an example helper stat validation function.
*   Change the way scope joins are done (GH-26).
*   Reorder some structs to avoid middle padding.

## 3.0.3 2016-02-18
*   make sampler function tunable (GH-24)

## 3.0.2 2016-01-13
*   reduce memory allocations
*   improve performance of buffered clients

## 3.0.1 2016-01-01
*   documentation typo fixes
*   fix possible race condition with `buffered_sender` send/close.

## 3.0.0 2015-12-04
*   add substatter support

## 2.0.2 2015-10-16
*   remove trailing newline in buffered sends to avoid etsy statsd log messages
*   minor internal code reorganization for clarity (no api changes)

## 2.0.1 2015-07-12
*   Add Set and SetInt funcs to support Sets
*   Properly flush BufferedSender on close (bugfix)
*   Add TimingDuration with support for sub-millisecond timing
*   fewer allocations, better performance of BufferedClient

## 2.0.0 2015-03-19
*   BufferedClient - send multiple stats at once
*   clean up godocs
*   clean up interfaces -- BREAKING CHANGE: for users who previously defined
    types as *Client instead of the Statter interface type.

## 1.0.1 2015-03-19
*   BufferedClient - send multiple stats at once

## 1.0.0 2015-02-04
*   tag a version as fix for GH-8
