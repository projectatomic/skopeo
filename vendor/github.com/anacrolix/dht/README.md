# dht

[![CircleCI](https://circleci.com/gh/anacrolix/dht.svg?style=shield)](https://circleci.com/gh/anacrolix/dht)
[![GoDoc](https://godoc.org/github.com/anacrolix/dht?status.svg)](https://godoc.org/github.com/anacrolix/dht)
[![Join the chat at https://gitter.im/anacrolix/torrent](https://badges.gitter.im/Join%20Chat.svg)](https://gitter.im/anacrolix/torrent?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

## Installation

Install the library package with `go get github.com/anacrolix/dht`, or the provided cmds with `go get github.com/anacrolix/dht/cmd/...`.

## Commands

Here I'll describe what some of the provided commands in `./cmd` do.

Note that the [`godo`](https://github.com/anacrolix/godo) command which is invoked in the following examples builds and executes a Go import path, like `go run`. It's easier to use this convention than to spell out the install/invoke cycle for every single example.

### dht-ping

Pings DHT nodes with the given network addresses.

    $ godo ./cmd/dht-ping router.bittorrent.com:6881 router.utorrent.com:6881
    2015/04/01 17:21:23 main.go:33: dht server on [::]:60058
    32f54e697351ff4aec29cdbaabf2fbe3467cc267 (router.bittorrent.com:6881): 648.218621ms
    ebff36697351ff4aec29cdbaabf2fbe3467cc267 (router.utorrent.com:6881): 873.864706ms
    2/2 responses (100.000000%)
