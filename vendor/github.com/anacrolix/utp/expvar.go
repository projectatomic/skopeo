package utp

import "expvar"

var (
	ackSkippedResends = expvar.NewInt("utpAckSkippedResends")
	// Inbound packets processed by a Conn.
	deliveriesProcessed    = expvar.NewInt("utpDeliveriesProcessed")
	sentStatePackets       = expvar.NewInt("utpSentStatePackets")
	acksReceivedAheadOfSyn = expvar.NewInt("utpAcksReceivedAheadOfSyn")
	unexpectedPacketsRead  = expvar.NewInt("utpUnexpectedPacketsRead")
	// State packets that we managed not to send.
	unsentStatePackets = expvar.NewInt("utpUnsentStatePackets")
	unusedReads        = expvar.NewInt("utpUnusedReads")
	unusedReadsDropped = expvar.NewInt("utpUnusedReadsDropped")

	largestReceivedUTPPacket       int
	largestReceivedUTPPacketExpvar = expvar.NewInt("utpLargestReceivedPacket")
)
