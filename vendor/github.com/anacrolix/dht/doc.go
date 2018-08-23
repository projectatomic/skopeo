// Package dht implements a Distributed Hash Table (DHT) part of
// the BitTorrent protocol,
// as specified by BEP 5: http://www.bittorrent.org/beps/bep_0005.html
//
// BitTorrent uses a "distributed hash table" (DHT)
// for storing peer contact information for "trackerless" torrents.
// In effect, each peer becomes a tracker.
// The protocol is based on Kademila DHT protocol and is implemented over UDP.
//
// Please note the terminology used to avoid confusion.
// A "peer" is a client/server listening on a TCP port that
// implements the BitTorrent protocol.
// A "node" is a client/server listening on a UDP port implementing
// the distributed hash table protocol.
// The DHT is composed of nodes and stores the location of peers.
// BitTorrent clients include a DHT node, which is used to contact other nodes
// in the DHT to get the location of peers to
// download from using the BitTorrent protocol.
//
// Standard use involves creating a Server, and calling Announce on it with
// the details of your local torrent client and infohash of interest.
package dht
