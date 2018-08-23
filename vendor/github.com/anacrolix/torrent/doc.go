/*
Package torrent implements a torrent client. Goals include:
 * Configurable data storage, such as file, mmap, and piece-based.
 * Downloading on demand: torrent.Reader will request only the data required to
   satisfy Reads, which is ideal for streaming and torrentfs.

BitTorrent features implemented include:
 * Protocol obfuscation
 * DHT
 * uTP
 * PEX
 * Magnet links
 * IP Blocklists
 * Some IPv6
 * HTTP and UDP tracker clients
 * BEPs:
  -  3: Basic BitTorrent protocol
  -  5: DHT
  -  6: Fast Extension (have all/none only)
  -  7: IPv6 Tracker Extension
  -  9: ut_metadata
  - 10: Extension protocol
  - 11: PEX
  - 12: Multitracker metadata extension
  - 15: UDP Tracker Protocol
  - 20: Peer ID convention ("-GTnnnn-")
  - 23: Tracker Returns Compact Peer Lists
  - 27: Private torrents
  - 29: uTorrent transport protocol
  - 41: UDP Tracker Protocol Extensions
  - 42: DHT Security extension
  - 43: Read-only DHT Nodes
*/
package torrent
