# utp
[![GoDoc](https://godoc.org/github.com/anacrolix/utp?status.svg)](https://godoc.org/github.com/anacrolix/utp)
[![CircleCI](https://circleci.com/gh/anacrolix/utp.svg?style=shield)](https://circleci.com/gh/anacrolix/utp)

Package utp implements uTP, the micro transport protocol as used with Bittorrent. It opts for simplicity and reliability over strict adherence to the (poor) spec.

## Supported

 * Multiple uTP connections switched on a single PacketConn, including those initiated locally.
 * Raw access to the PacketConn for non-uTP purposes, like sharing the PacketConn with a DHT implementation.

## Implementation characteristics

 * There is no MTU path discovery.
 * A fixed 64 slot selective ack window is used in both sending and receiving.

Patches welcomed.
