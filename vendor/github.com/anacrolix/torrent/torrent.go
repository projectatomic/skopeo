package torrent

import (
	"container/heap"
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net"
	"net/url"
	"os"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/anacrolix/dht"
	"github.com/anacrolix/log"
	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/missinggo/bitmap"
	"github.com/anacrolix/missinggo/perf"
	"github.com/anacrolix/missinggo/prioritybitmap"
	"github.com/anacrolix/missinggo/pubsub"
	"github.com/anacrolix/missinggo/slices"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	pp "github.com/anacrolix/torrent/peer_protocol"
	"github.com/anacrolix/torrent/storage"
	"github.com/anacrolix/torrent/tracker"
	"github.com/davecgh/go-spew/spew"
)

func (t *Torrent) chunkIndexSpec(chunkIndex pp.Integer, piece pieceIndex) chunkSpec {
	return chunkIndexSpec(chunkIndex, t.pieceLength(piece), t.chunkSize)
}

// Maintains state of torrent within a Client.
type Torrent struct {
	// Torrent-level aggregate statistics. First in struct to ensure 64-bit
	// alignment. See #262.
	stats  ConnStats
	cl     *Client
	logger *log.Logger

	networkingEnabled bool

	// Determines what chunks to request from peers. 1: Favour higher priority
	// pieces with some fuzzing to reduce overlaps and wastage across
	// connections. 2: The fastest connection downloads strictly in order of
	// priority, while all others adher to their piece inclications. 3:
	// Requests are strictly by piece priority, and not duplicated until
	// duplicateRequestTimeout is reached.
	requestStrategy int
	// How long to avoid duplicating a pending request.
	duplicateRequestTimeout time.Duration

	closed   missinggo.Event
	infoHash metainfo.Hash
	pieces   []Piece
	// Values are the piece indices that changed.
	pieceStateChanges *pubsub.PubSub
	// The size of chunks to request from peers over the wire. This is
	// normally 16KiB by convention these days.
	chunkSize pp.Integer
	chunkPool *sync.Pool
	// Total length of the torrent in bytes. Stored because it's not O(1) to
	// get this from the info dict.
	length *int64

	// The storage to open when the info dict becomes available.
	storageOpener *storage.Client
	// Storage for torrent data.
	storage *storage.Torrent
	// Read-locked for using storage, and write-locked for Closing.
	storageLock sync.RWMutex

	// TODO: Only announce stuff is used?
	metainfo metainfo.MetaInfo

	// The info dict. nil if we don't have it (yet).
	info  *metainfo.Info
	files *[]*File

	// Active peer connections, running message stream loops. TODO: Make this
	// open (not-closed) connections only.
	conns               map[*connection]struct{}
	maxEstablishedConns int
	// Set of addrs to which we're attempting to connect. Connections are
	// half-open until all handshakes are completed.
	halfOpen    map[string]Peer
	fastestConn *connection

	// Reserve of peers to connect to. A peer can be both here and in the
	// active connections if were told about the peer after connecting with
	// them. That encourages us to reconnect to peers that are well known in
	// the swarm.
	peers          prioritizedPeers
	wantPeersEvent missinggo.Event
	// An announcer for each tracker URL.
	trackerAnnouncers map[string]*trackerScraper
	// How many times we've initiated a DHT announce. TODO: Move into stats.
	numDHTAnnounces int

	// Name used if the info name isn't available. Should be cleared when the
	// Info does become available.
	displayName string

	// The bencoded bytes of the info dict. This is actively manipulated if
	// the info bytes aren't initially available, and we try to fetch them
	// from peers.
	metadataBytes []byte
	// Each element corresponds to the 16KiB metadata pieces. If true, we have
	// received that piece.
	metadataCompletedChunks []bool
	metadataChanged         sync.Cond

	// Set when .Info is obtained.
	gotMetainfo missinggo.Event

	readers               map[*reader]struct{}
	readerNowPieces       bitmap.Bitmap
	readerReadaheadPieces bitmap.Bitmap

	// A cache of pieces we need to get. Calculated from various piece and
	// file priorities and completion states elsewhere.
	pendingPieces prioritybitmap.PriorityBitmap
	// A cache of completed piece indices.
	completedPieces bitmap.Bitmap
	// Pieces that need to be hashed.
	piecesQueuedForHash bitmap.Bitmap

	// A pool of piece priorities []int for assignment to new connections.
	// These "inclinations" are used to give connections preference for
	// different pieces.
	connPieceInclinationPool sync.Pool

	// Count of each request across active connections.
	pendingRequests map[request]int
	// The last time we requested a chunk. Deleting the request from any
	// connection will clear this value.
	lastRequested map[request]*time.Timer
}

func (t *Torrent) tickleReaders() {
	t.cl.event.Broadcast()
}

// Returns a channel that is closed when the Torrent is closed.
func (t *Torrent) Closed() <-chan struct{} {
	return t.closed.LockedChan(t.cl.locker())
}

// KnownSwarm returns the known subset of the peers in the Torrent's swarm, including active,
// pending, and half-open peers.
func (t *Torrent) KnownSwarm() (ks []Peer) {
	// Add pending peers to the list
	t.peers.Each(func(peer Peer) {
		ks = append(ks, peer)
	})

	// Add half-open peers to the list
	for _, peer := range t.halfOpen {
		ks = append(ks, peer)
	}

	// Add active peers to the list
	for conn := range t.conns {

		ks = append(ks, Peer{
			Id:     conn.PeerID,
			IP:     conn.remoteAddr.IP,
			Port:   int(conn.remoteAddr.Port),
			Source: conn.Discovery,
			// > If the connection is encrypted, that's certainly enough to set SupportsEncryption.
			// > But if we're not connected to them with an encrypted connection, I couldn't say
			// > what's appropriate. We can carry forward the SupportsEncryption value as we
			// > received it from trackers/DHT/PEX, or just use the encryption state for the
			// > connection. It's probably easiest to do the latter for now.
			// https://github.com/anacrolix/torrent/pull/188
			SupportsEncryption: conn.headerEncrypted,
		})
	}

	return
}

func (t *Torrent) setChunkSize(size pp.Integer) {
	t.chunkSize = size
	t.chunkPool = &sync.Pool{
		New: func() interface{} {
			b := make([]byte, size)
			return &b
		},
	}
}

func (t *Torrent) setDisplayName(dn string) {
	if t.haveInfo() {
		return
	}
	t.displayName = dn
}

func (t *Torrent) pieceComplete(piece pieceIndex) bool {
	return t.completedPieces.Get(bitmap.BitIndex(piece))
}

func (t *Torrent) pieceCompleteUncached(piece pieceIndex) storage.Completion {
	return t.pieces[piece].Storage().Completion()
}

// There's a connection to that address already.
func (t *Torrent) addrActive(addr string) bool {
	if _, ok := t.halfOpen[addr]; ok {
		return true
	}
	for c := range t.conns {
		ra := c.remoteAddr
		if ra.String() == addr {
			return true
		}
	}
	return false
}

func (t *Torrent) unclosedConnsAsSlice() (ret []*connection) {
	ret = make([]*connection, 0, len(t.conns))
	for c := range t.conns {
		if !c.closed.IsSet() {
			ret = append(ret, c)
		}
	}
	return
}

func (t *Torrent) addPeer(p Peer) {
	cl := t.cl
	peersAddedBySource.Add(string(p.Source), 1)
	if t.closed.IsSet() {
		return
	}
	if cl.badPeerIPPort(p.IP, p.Port) {
		torrent.Add("peers not added because of bad addr", 1)
		return
	}
	if t.peers.Add(p) {
		torrent.Add("peers replaced", 1)
	}
	t.openNewConns()
	for t.peers.Len() > cl.config.TorrentPeersHighWater {
		_, ok := t.peers.DeleteMin()
		if ok {
			torrent.Add("excess reserve peers discarded", 1)
		}
	}
}

func (t *Torrent) invalidateMetadata() {
	for i := range t.metadataCompletedChunks {
		t.metadataCompletedChunks[i] = false
	}
	t.info = nil
}

func (t *Torrent) saveMetadataPiece(index int, data []byte) {
	if t.haveInfo() {
		return
	}
	if index >= len(t.metadataCompletedChunks) {
		log.Printf("%s: ignoring metadata piece %d", t, index)
		return
	}
	copy(t.metadataBytes[(1<<14)*index:], data)
	t.metadataCompletedChunks[index] = true
}

func (t *Torrent) metadataPieceCount() int {
	return (len(t.metadataBytes) + (1 << 14) - 1) / (1 << 14)
}

func (t *Torrent) haveMetadataPiece(piece int) bool {
	if t.haveInfo() {
		return (1<<14)*piece < len(t.metadataBytes)
	} else {
		return piece < len(t.metadataCompletedChunks) && t.metadataCompletedChunks[piece]
	}
}

func (t *Torrent) metadataSize() int {
	return len(t.metadataBytes)
}

func infoPieceHashes(info *metainfo.Info) (ret []string) {
	for i := 0; i < len(info.Pieces); i += sha1.Size {
		ret = append(ret, string(info.Pieces[i:i+sha1.Size]))
	}
	return
}

func (t *Torrent) makePieces() {
	hashes := infoPieceHashes(t.info)
	t.pieces = make([]Piece, len(hashes), len(hashes))
	for i, hash := range hashes {
		piece := &t.pieces[i]
		piece.t = t
		piece.index = pieceIndex(i)
		piece.noPendingWrites.L = &piece.pendingWritesMutex
		missinggo.CopyExact(piece.hash[:], hash)
		files := *t.files
		beginFile := pieceFirstFileIndex(piece.torrentBeginOffset(), files)
		endFile := pieceEndFileIndex(piece.torrentEndOffset(), files)
		piece.files = files[beginFile:endFile]
	}
}

// Returns the index of the first file containing the piece. files must be
// ordered by offset.
func pieceFirstFileIndex(pieceOffset int64, files []*File) int {
	for i, f := range files {
		if f.offset+f.length > pieceOffset {
			return i
		}
	}
	return 0
}

// Returns the index after the last file containing the piece. files must be
// ordered by offset.
func pieceEndFileIndex(pieceEndOffset int64, files []*File) int {
	for i, f := range files {
		if f.offset+f.length >= pieceEndOffset {
			return i + 1
		}
	}
	return 0
}

func (t *Torrent) cacheLength() {
	var l int64
	for _, f := range t.info.UpvertedFiles() {
		l += f.Length
	}
	t.length = &l
}

func (t *Torrent) setInfo(info *metainfo.Info) error {
	if err := validateInfo(info); err != nil {
		return fmt.Errorf("bad info: %s", err)
	}
	if t.storageOpener != nil {
		var err error
		t.storage, err = t.storageOpener.OpenTorrent(info, t.infoHash)
		if err != nil {
			return fmt.Errorf("error opening torrent storage: %s", err)
		}
	}
	t.info = info
	t.displayName = "" // Save a few bytes lol.
	t.initFiles()
	t.cacheLength()
	t.makePieces()
	return nil
}

func (t *Torrent) onSetInfo() {
	for conn := range t.conns {
		if err := conn.setNumPieces(t.numPieces()); err != nil {
			log.Printf("closing connection: %s", err)
			conn.Close()
		}
	}
	for i := range t.pieces {
		t.updatePieceCompletion(pieceIndex(i))
		p := &t.pieces[i]
		if !p.storageCompletionOk {
			// log.Printf("piece %s completion unknown, queueing check", p)
			t.queuePieceCheck(pieceIndex(i))
		}
	}
	t.cl.event.Broadcast()
	t.gotMetainfo.Set()
	t.updateWantPeersEvent()
	t.pendingRequests = make(map[request]int)
	t.lastRequested = make(map[request]*time.Timer)
}

// Called when metadata for a torrent becomes available.
func (t *Torrent) setInfoBytes(b []byte) error {
	if metainfo.HashBytes(b) != t.infoHash {
		return errors.New("info bytes have wrong hash")
	}
	var info metainfo.Info
	if err := bencode.Unmarshal(b, &info); err != nil {
		return fmt.Errorf("error unmarshalling info bytes: %s", err)
	}
	if err := t.setInfo(&info); err != nil {
		return err
	}
	t.metadataBytes = b
	t.metadataCompletedChunks = nil
	t.onSetInfo()
	return nil
}

func (t *Torrent) haveAllMetadataPieces() bool {
	if t.haveInfo() {
		return true
	}
	if t.metadataCompletedChunks == nil {
		return false
	}
	for _, have := range t.metadataCompletedChunks {
		if !have {
			return false
		}
	}
	return true
}

// TODO: Propagate errors to disconnect peer.
func (t *Torrent) setMetadataSize(bytes int) (err error) {
	if t.haveInfo() {
		// We already know the correct metadata size.
		return
	}
	if bytes <= 0 || bytes > 10000000 { // 10MB, pulled from my ass.
		return errors.New("bad size")
	}
	if t.metadataBytes != nil && len(t.metadataBytes) == int(bytes) {
		return
	}
	t.metadataBytes = make([]byte, bytes)
	t.metadataCompletedChunks = make([]bool, (bytes+(1<<14)-1)/(1<<14))
	t.metadataChanged.Broadcast()
	for c := range t.conns {
		c.requestPendingMetadata()
	}
	return
}

// The current working name for the torrent. Either the name in the info dict,
// or a display name given such as by the dn value in a magnet link, or "".
func (t *Torrent) name() string {
	if t.haveInfo() {
		return t.info.Name
	}
	return t.displayName
}

func (t *Torrent) pieceState(index pieceIndex) (ret PieceState) {
	p := &t.pieces[index]
	ret.Priority = t.piecePriority(index)
	ret.Completion = p.completion()
	if p.queuedForHash() || p.hashing {
		ret.Checking = true
	}
	if !ret.Complete && t.piecePartiallyDownloaded(index) {
		ret.Partial = true
	}
	return
}

func (t *Torrent) metadataPieceSize(piece int) int {
	return metadataPieceSize(len(t.metadataBytes), piece)
}

func (t *Torrent) newMetadataExtensionMessage(c *connection, msgType int, piece int, data []byte) pp.Message {
	d := map[string]int{
		"msg_type": msgType,
		"piece":    piece,
	}
	if data != nil {
		d["total_size"] = len(t.metadataBytes)
	}
	p := bencode.MustMarshal(d)
	return pp.Message{
		Type:            pp.Extended,
		ExtendedID:      c.PeerExtensionIDs[pp.ExtensionNameMetadata],
		ExtendedPayload: append(p, data...),
	}
}

func (t *Torrent) pieceStateRuns() (ret []PieceStateRun) {
	rle := missinggo.NewRunLengthEncoder(func(el interface{}, count uint64) {
		ret = append(ret, PieceStateRun{
			PieceState: el.(PieceState),
			Length:     int(count),
		})
	})
	for index := range t.pieces {
		rle.Append(t.pieceState(pieceIndex(index)), 1)
	}
	rle.Flush()
	return
}

// Produces a small string representing a PieceStateRun.
func pieceStateRunStatusChars(psr PieceStateRun) (ret string) {
	ret = fmt.Sprintf("%d", psr.Length)
	ret += func() string {
		switch psr.Priority {
		case PiecePriorityNext:
			return "N"
		case PiecePriorityNormal:
			return "."
		case PiecePriorityReadahead:
			return "R"
		case PiecePriorityNow:
			return "!"
		case PiecePriorityHigh:
			return "H"
		default:
			return ""
		}
	}()
	if psr.Checking {
		ret += "H"
	}
	if psr.Partial {
		ret += "P"
	}
	if psr.Complete {
		ret += "C"
	}
	if !psr.Ok {
		ret += "?"
	}
	return
}

func (t *Torrent) writeStatus(w io.Writer) {
	fmt.Fprintf(w, "Infohash: %s\n", t.infoHash.HexString())
	fmt.Fprintf(w, "Metadata length: %d\n", t.metadataSize())
	if !t.haveInfo() {
		fmt.Fprintf(w, "Metadata have: ")
		for _, h := range t.metadataCompletedChunks {
			fmt.Fprintf(w, "%c", func() rune {
				if h {
					return 'H'
				} else {
					return '.'
				}
			}())
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "Piece length: %s\n", func() string {
		if t.haveInfo() {
			return fmt.Sprint(t.usualPieceSize())
		} else {
			return "?"
		}
	}())
	if t.info != nil {
		fmt.Fprintf(w, "Num Pieces: %d (%d completed)\n", t.numPieces(), t.numPiecesCompleted())
		fmt.Fprint(w, "Piece States:")
		for _, psr := range t.pieceStateRuns() {
			w.Write([]byte(" "))
			w.Write([]byte(pieceStateRunStatusChars(psr)))
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "Reader Pieces:")
	t.forReaderOffsetPieces(func(begin, end pieceIndex) (again bool) {
		fmt.Fprintf(w, " %d:%d", begin, end)
		return true
	})
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Enabled trackers:\n")
	func() {
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "    URL\tNext announce\tLast announce\n")
		for _, ta := range slices.Sort(slices.FromMapElems(t.trackerAnnouncers), func(l, r *trackerScraper) bool {
			return l.u.String() < r.u.String()
		}).([]*trackerScraper) {
			fmt.Fprintf(tw, "    %s\n", ta.statusLine())
		}
		tw.Flush()
	}()

	fmt.Fprintf(w, "DHT Announces: %d\n", t.numDHTAnnounces)

	spew.NewDefaultConfig()
	spew.Fdump(w, t.statsLocked())

	conns := t.connsAsSlice()
	slices.Sort(conns, worseConn)
	for i, c := range conns {
		fmt.Fprintf(w, "%2d. ", i+1)
		c.WriteStatus(w, t)
	}
}

func (t *Torrent) haveInfo() bool {
	return t.info != nil
}

// Returns a run-time generated MetaInfo that includes the info bytes and
// announce-list as currently known to the client.
func (t *Torrent) newMetaInfo() metainfo.MetaInfo {
	return metainfo.MetaInfo{
		CreationDate: time.Now().Unix(),
		Comment:      "dynamic metainfo from client",
		CreatedBy:    "go.torrent",
		AnnounceList: t.metainfo.UpvertedAnnounceList(),
		InfoBytes: func() []byte {
			if t.haveInfo() {
				return t.metadataBytes
			} else {
				return nil
			}
		}(),
	}
}

func (t *Torrent) BytesMissing() int64 {
	t.cl.rLock()
	defer t.cl.rUnlock()
	return t.bytesMissingLocked()
}

func (t *Torrent) bytesMissingLocked() int64 {
	return t.bytesLeft()
}

func (t *Torrent) bytesLeft() (left int64) {
	bitmap.Flip(t.completedPieces, 0, bitmap.BitIndex(t.numPieces())).IterTyped(func(piece int) bool {
		p := &t.pieces[piece]
		left += int64(p.length() - p.numDirtyBytes())
		return true
	})
	return
}

// Bytes left to give in tracker announces.
func (t *Torrent) bytesLeftAnnounce() uint64 {
	if t.haveInfo() {
		return uint64(t.bytesLeft())
	} else {
		return math.MaxUint64
	}
}

func (t *Torrent) piecePartiallyDownloaded(piece pieceIndex) bool {
	if t.pieceComplete(piece) {
		return false
	}
	if t.pieceAllDirty(piece) {
		return false
	}
	return t.pieces[piece].hasDirtyChunks()
}

func (t *Torrent) usualPieceSize() int {
	return int(t.info.PieceLength)
}

func (t *Torrent) numPieces() pieceIndex {
	return pieceIndex(t.info.NumPieces())
}

func (t *Torrent) numPiecesCompleted() (num int) {
	return t.completedPieces.Len()
}

func (t *Torrent) close() (err error) {
	t.closed.Set()
	t.tickleReaders()
	if t.storage != nil {
		t.storageLock.Lock()
		t.storage.Close()
		t.storageLock.Unlock()
	}
	for conn := range t.conns {
		conn.Close()
	}
	t.cl.event.Broadcast()
	t.pieceStateChanges.Close()
	t.updateWantPeersEvent()
	return
}

func (t *Torrent) requestOffset(r request) int64 {
	return torrentRequestOffset(*t.length, int64(t.usualPieceSize()), r)
}

// Return the request that would include the given offset into the torrent
// data. Returns !ok if there is no such request.
func (t *Torrent) offsetRequest(off int64) (req request, ok bool) {
	return torrentOffsetRequest(*t.length, t.info.PieceLength, int64(t.chunkSize), off)
}

func (t *Torrent) writeChunk(piece int, begin int64, data []byte) (err error) {
	defer perf.ScopeTimerErr(&err)()
	n, err := t.pieces[piece].Storage().WriteAt(data, begin)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return
}

func (t *Torrent) bitfield() (bf []bool) {
	bf = make([]bool, t.numPieces())
	t.completedPieces.IterTyped(func(piece int) (again bool) {
		bf[piece] = true
		return true
	})
	return
}

func (t *Torrent) pieceNumChunks(piece pieceIndex) pp.Integer {
	return (t.pieceLength(piece) + t.chunkSize - 1) / t.chunkSize
}

func (t *Torrent) pendAllChunkSpecs(pieceIndex pieceIndex) {
	t.pieces[pieceIndex].dirtyChunks.Clear()
}

func (t *Torrent) pieceLength(piece pieceIndex) pp.Integer {
	if t.info.PieceLength == 0 {
		// There will be no variance amongst pieces. Only pain.
		return 0
	}
	if piece == t.numPieces()-1 {
		ret := pp.Integer(*t.length % t.info.PieceLength)
		if ret != 0 {
			return ret
		}
	}
	return pp.Integer(t.info.PieceLength)
}

func (t *Torrent) hashPiece(piece pieceIndex) (ret metainfo.Hash) {
	hash := pieceHash.New()
	p := &t.pieces[piece]
	p.waitNoPendingWrites()
	ip := t.info.Piece(int(piece))
	pl := ip.Length()
	n, err := io.Copy(hash, io.NewSectionReader(t.pieces[piece].Storage(), 0, pl))
	if n == pl {
		missinggo.CopyExact(&ret, hash.Sum(nil))
		return
	}
	if err != io.ErrUnexpectedEOF && !os.IsNotExist(err) {
		log.Printf("unexpected error hashing piece with %T: %s", t.storage.TorrentImpl, err)
	}
	return
}

func (t *Torrent) haveAnyPieces() bool {
	return t.completedPieces.Len() != 0
}

func (t *Torrent) haveAllPieces() bool {
	if !t.haveInfo() {
		return false
	}
	return t.completedPieces.Len() == bitmap.BitIndex(t.numPieces())
}

func (t *Torrent) havePiece(index pieceIndex) bool {
	return t.haveInfo() && t.pieceComplete(index)
}

func (t *Torrent) haveChunk(r request) (ret bool) {
	// defer func() {
	// 	log.Println("have chunk", r, ret)
	// }()
	if !t.haveInfo() {
		return false
	}
	if t.pieceComplete(pieceIndex(r.Index)) {
		return true
	}
	p := &t.pieces[r.Index]
	return !p.pendingChunk(r.chunkSpec, t.chunkSize)
}

func chunkIndex(cs chunkSpec, chunkSize pp.Integer) int {
	return int(cs.Begin / chunkSize)
}

func (t *Torrent) wantPieceIndex(index pieceIndex) bool {
	if !t.haveInfo() {
		return false
	}
	if index < 0 || index >= t.numPieces() {
		return false
	}
	p := &t.pieces[index]
	if p.queuedForHash() {
		return false
	}
	if p.hashing {
		return false
	}
	if t.pieceComplete(index) {
		return false
	}
	if t.pendingPieces.Contains(bitmap.BitIndex(index)) {
		return true
	}
	// log.Printf("piece %d not pending", index)
	return !t.forReaderOffsetPieces(func(begin, end pieceIndex) bool {
		return index < begin || index >= end
	})
}

// The worst connection is one that hasn't been sent, or sent anything useful
// for the longest. A bad connection is one that usually sends us unwanted
// pieces, or has been in worser half of the established connections for more
// than a minute.
func (t *Torrent) worstBadConn() *connection {
	wcs := worseConnSlice{t.unclosedConnsAsSlice()}
	heap.Init(&wcs)
	for wcs.Len() != 0 {
		c := heap.Pop(&wcs).(*connection)
		if c.stats.ChunksReadWasted.Int64() >= 6 && c.stats.ChunksReadWasted.Int64() > c.stats.ChunksReadUseful.Int64() {
			return c
		}
		// If the connection is in the worst half of the established
		// connection quota and is older than a minute.
		if wcs.Len() >= (t.maxEstablishedConns+1)/2 {
			// Give connections 1 minute to prove themselves.
			if time.Since(c.completedHandshake) > time.Minute {
				return c
			}
		}
	}
	return nil
}

type PieceStateChange struct {
	Index int
	PieceState
}

func (t *Torrent) publishPieceChange(piece pieceIndex) {
	cur := t.pieceState(piece)
	p := &t.pieces[piece]
	if cur != p.publicPieceState {
		p.publicPieceState = cur
		t.pieceStateChanges.Publish(PieceStateChange{
			int(piece),
			cur,
		})
	}
}

func (t *Torrent) pieceNumPendingChunks(piece pieceIndex) pp.Integer {
	if t.pieceComplete(piece) {
		return 0
	}
	return t.pieceNumChunks(piece) - t.pieces[piece].numDirtyChunks()
}

func (t *Torrent) pieceAllDirty(piece pieceIndex) bool {
	return t.pieces[piece].dirtyChunks.Len() == int(t.pieceNumChunks(piece))
}

func (t *Torrent) readersChanged() {
	t.updateReaderPieces()
	t.updateAllPiecePriorities()
}

func (t *Torrent) updateReaderPieces() {
	t.readerNowPieces, t.readerReadaheadPieces = t.readerPiecePriorities()
}

func (t *Torrent) readerPosChanged(from, to pieceRange) {
	if from == to {
		return
	}
	t.updateReaderPieces()
	// Order the ranges, high and low.
	l, h := from, to
	if l.begin > h.begin {
		l, h = h, l
	}
	if l.end < h.begin {
		// Two distinct ranges.
		t.updatePiecePriorities(l.begin, l.end)
		t.updatePiecePriorities(h.begin, h.end)
	} else {
		// Ranges overlap.
		end := l.end
		if h.end > end {
			end = h.end
		}
		t.updatePiecePriorities(l.begin, end)
	}
}

func (t *Torrent) maybeNewConns() {
	// Tickle the accept routine.
	t.cl.event.Broadcast()
	t.openNewConns()
}

func (t *Torrent) piecePriorityChanged(piece pieceIndex) {
	// log.Printf("piece %d priority changed", piece)
	for c := range t.conns {
		if c.updatePiecePriority(piece) {
			// log.Print("conn piece priority changed")
			c.updateRequests()
		}
	}
	t.maybeNewConns()
	t.publishPieceChange(piece)
}

func (t *Torrent) updatePiecePriority(piece pieceIndex) {
	p := &t.pieces[piece]
	newPrio := p.uncachedPriority()
	// log.Printf("torrent %p: piece %d: uncached priority: %v", t, piece, newPrio)
	if newPrio == PiecePriorityNone {
		if !t.pendingPieces.Remove(bitmap.BitIndex(piece)) {
			return
		}
	} else {
		if !t.pendingPieces.Set(bitmap.BitIndex(piece), newPrio.BitmapPriority()) {
			return
		}
	}
	t.piecePriorityChanged(piece)
}

func (t *Torrent) updateAllPiecePriorities() {
	t.updatePiecePriorities(0, t.numPieces())
}

// Update all piece priorities in one hit. This function should have the same
// output as updatePiecePriority, but across all pieces.
func (t *Torrent) updatePiecePriorities(begin, end pieceIndex) {
	for i := begin; i < end; i++ {
		t.updatePiecePriority(i)
	}
}

// Returns the range of pieces [begin, end) that contains the extent of bytes.
func (t *Torrent) byteRegionPieces(off, size int64) (begin, end pieceIndex) {
	if off >= *t.length {
		return
	}
	if off < 0 {
		size += off
		off = 0
	}
	if size <= 0 {
		return
	}
	begin = pieceIndex(off / t.info.PieceLength)
	end = pieceIndex((off + size + t.info.PieceLength - 1) / t.info.PieceLength)
	if end > pieceIndex(t.info.NumPieces()) {
		end = pieceIndex(t.info.NumPieces())
	}
	return
}

// Returns true if all iterations complete without breaking. Returns the read
// regions for all readers. The reader regions should not be merged as some
// callers depend on this method to enumerate readers.
func (t *Torrent) forReaderOffsetPieces(f func(begin, end pieceIndex) (more bool)) (all bool) {
	for r := range t.readers {
		p := r.pieces
		if p.begin >= p.end {
			continue
		}
		if !f(p.begin, p.end) {
			return false
		}
	}
	return true
}

func (t *Torrent) piecePriority(piece pieceIndex) piecePriority {
	prio, ok := t.pendingPieces.GetPriority(bitmap.BitIndex(piece))
	if !ok {
		return PiecePriorityNone
	}
	if prio > 0 {
		panic(prio)
	}
	ret := piecePriority(-prio)
	if ret == PiecePriorityNone {
		panic(piece)
	}
	return ret
}

func (t *Torrent) pendRequest(req request) {
	ci := chunkIndex(req.chunkSpec, t.chunkSize)
	t.pieces[req.Index].pendChunkIndex(ci)
}

func (t *Torrent) pieceCompletionChanged(piece pieceIndex) {
	log.Call().Add("piece", piece).AddValue(debugLogValue).Log(t.logger)
	t.cl.event.Broadcast()
	if t.pieceComplete(piece) {
		t.onPieceCompleted(piece)
	} else {
		t.onIncompletePiece(piece)
	}
	t.updatePiecePriority(piece)
}

func (t *Torrent) numReceivedConns() (ret int) {
	for c := range t.conns {
		if c.Discovery == peerSourceIncoming {
			ret++
		}
	}
	return
}

func (t *Torrent) maxHalfOpen() int {
	// Note that if we somehow exceed the maximum established conns, we want
	// the negative value to have an effect.
	establishedHeadroom := int64(t.maxEstablishedConns - len(t.conns))
	extraIncoming := int64(t.numReceivedConns() - t.maxEstablishedConns/2)
	// We want to allow some experimentation with new peers, and to try to
	// upset an oversupply of received connections.
	return int(min(max(5, extraIncoming)+establishedHeadroom, int64(t.cl.config.HalfOpenConnsPerTorrent)))
}

func (t *Torrent) openNewConns() {
	defer t.updateWantPeersEvent()
	for t.peers.Len() != 0 {
		if !t.wantConns() {
			return
		}
		if len(t.halfOpen) >= t.maxHalfOpen() {
			return
		}
		p := t.peers.PopMax()
		t.initiateConn(p)
	}
}

func (t *Torrent) getConnPieceInclination() []int {
	_ret := t.connPieceInclinationPool.Get()
	if _ret == nil {
		pieceInclinationsNew.Add(1)
		return rand.Perm(int(t.numPieces()))
	}
	pieceInclinationsReused.Add(1)
	return *_ret.(*[]int)
}

func (t *Torrent) putPieceInclination(pi []int) {
	t.connPieceInclinationPool.Put(&pi)
	pieceInclinationsPut.Add(1)
}

func (t *Torrent) updatePieceCompletion(piece pieceIndex) {
	pcu := t.pieceCompleteUncached(piece)
	p := &t.pieces[piece]
	changed := t.completedPieces.Get(bitmap.BitIndex(piece)) != pcu.Complete || p.storageCompletionOk != pcu.Ok
	log.Fmsg("piece %d completion: %v", piece, pcu.Ok).AddValue(debugLogValue).Log(t.logger)
	p.storageCompletionOk = pcu.Ok
	t.completedPieces.Set(bitmap.BitIndex(piece), pcu.Complete)
	t.tickleReaders()
	// log.Printf("piece %d uncached completion: %v", piece, pcu.Complete)
	// log.Printf("piece %d changed: %v", piece, changed)
	if changed {
		t.pieceCompletionChanged(piece)
	}
}

// Non-blocking read. Client lock is not required.
func (t *Torrent) readAt(b []byte, off int64) (n int, err error) {
	p := &t.pieces[off/t.info.PieceLength]
	p.waitNoPendingWrites()
	return p.Storage().ReadAt(b, off-p.Info().Offset())
}

func (t *Torrent) updateAllPieceCompletions() {
	for i := pieceIndex(0); i < t.numPieces(); i++ {
		t.updatePieceCompletion(i)
	}
}

// Returns an error if the metadata was completed, but couldn't be set for
// some reason. Blame it on the last peer to contribute.
func (t *Torrent) maybeCompleteMetadata() error {
	if t.haveInfo() {
		// Nothing to do.
		return nil
	}
	if !t.haveAllMetadataPieces() {
		// Don't have enough metadata pieces.
		return nil
	}
	err := t.setInfoBytes(t.metadataBytes)
	if err != nil {
		t.invalidateMetadata()
		return fmt.Errorf("error setting info bytes: %s", err)
	}
	if t.cl.config.Debug {
		log.Printf("%s: got metadata from peers", t)
	}
	return nil
}

func (t *Torrent) readerPiecePriorities() (now, readahead bitmap.Bitmap) {
	t.forReaderOffsetPieces(func(begin, end pieceIndex) bool {
		if end > begin {
			now.Add(bitmap.BitIndex(begin))
			readahead.AddRange(bitmap.BitIndex(begin)+1, bitmap.BitIndex(end))
		}
		return true
	})
	return
}

func (t *Torrent) needData() bool {
	if t.closed.IsSet() {
		return false
	}
	if !t.haveInfo() {
		return true
	}
	return t.pendingPieces.Len() != 0
}

func appendMissingStrings(old, new []string) (ret []string) {
	ret = old
new:
	for _, n := range new {
		for _, o := range old {
			if o == n {
				continue new
			}
		}
		ret = append(ret, n)
	}
	return
}

func appendMissingTrackerTiers(existing [][]string, minNumTiers int) (ret [][]string) {
	ret = existing
	for minNumTiers > len(ret) {
		ret = append(ret, nil)
	}
	return
}

func (t *Torrent) addTrackers(announceList [][]string) {
	fullAnnounceList := &t.metainfo.AnnounceList
	t.metainfo.AnnounceList = appendMissingTrackerTiers(*fullAnnounceList, len(announceList))
	for tierIndex, trackerURLs := range announceList {
		(*fullAnnounceList)[tierIndex] = appendMissingStrings((*fullAnnounceList)[tierIndex], trackerURLs)
	}
	t.startMissingTrackerScrapers()
	t.updateWantPeersEvent()
}

// Don't call this before the info is available.
func (t *Torrent) bytesCompleted() int64 {
	if !t.haveInfo() {
		return 0
	}
	return t.info.TotalLength() - t.bytesLeft()
}

func (t *Torrent) SetInfoBytes(b []byte) (err error) {
	t.cl.lock()
	defer t.cl.unlock()
	return t.setInfoBytes(b)
}

// Returns true if connection is removed from torrent.Conns.
func (t *Torrent) deleteConnection(c *connection) (ret bool) {
	if !c.closed.IsSet() {
		panic("connection is not closed")
		// There are behaviours prevented by the closed state that will fail
		// if the connection has been deleted.
	}
	_, ret = t.conns[c]
	delete(t.conns, c)
	torrent.Add("deleted connections", 1)
	c.deleteAllRequests()
	if len(t.conns) == 0 {
		t.assertNoPendingRequests()
	}
	return
}

func (t *Torrent) assertNoPendingRequests() {
	if len(t.pendingRequests) != 0 {
		panic(t.pendingRequests)
	}
	if len(t.lastRequested) != 0 {
		panic(t.lastRequested)
	}
}

func (t *Torrent) dropConnection(c *connection) {
	t.cl.event.Broadcast()
	c.Close()
	if t.deleteConnection(c) {
		t.openNewConns()
	}
}

func (t *Torrent) wantPeers() bool {
	if t.closed.IsSet() {
		return false
	}
	if t.peers.Len() > t.cl.config.TorrentPeersLowWater {
		return false
	}
	return t.needData() || t.seeding()
}

func (t *Torrent) updateWantPeersEvent() {
	if t.wantPeers() {
		t.wantPeersEvent.Set()
	} else {
		t.wantPeersEvent.Clear()
	}
}

// Returns whether the client should make effort to seed the torrent.
func (t *Torrent) seeding() bool {
	cl := t.cl
	if t.closed.IsSet() {
		return false
	}
	if cl.config.NoUpload {
		return false
	}
	if !cl.config.Seed {
		return false
	}
	if cl.config.DisableAggressiveUpload && t.needData() {
		return false
	}
	return true
}

func (t *Torrent) startScrapingTracker(_url string) {
	if _url == "" {
		return
	}
	u, err := url.Parse(_url)
	if err != nil {
		log.Str("error parsing tracker url").AddValues("url", _url).Log(t.logger)
		// TODO: Handle urls with leading '*', some kind of silly uTorrent
		// convention?
		return
	}
	if u.Scheme == "udp" {
		u.Scheme = "udp4"
		t.startScrapingTracker(u.String())
		u.Scheme = "udp6"
		t.startScrapingTracker(u.String())
		return
	}
	if u.Scheme == "udp4" && (t.cl.config.DisableIPv4Peers || t.cl.config.DisableIPv4) {
		return
	}
	if u.Scheme == "udp6" && t.cl.config.DisableIPv6 {
		return
	}
	if _, ok := t.trackerAnnouncers[_url]; ok {
		return
	}
	newAnnouncer := &trackerScraper{
		u: *u,
		t: t,
	}
	if t.trackerAnnouncers == nil {
		t.trackerAnnouncers = make(map[string]*trackerScraper)
	}
	t.trackerAnnouncers[_url] = newAnnouncer
	go newAnnouncer.Run()
}

// Adds and starts tracker scrapers for tracker URLs that aren't already
// running.
func (t *Torrent) startMissingTrackerScrapers() {
	if t.cl.config.DisableTrackers {
		return
	}
	t.startScrapingTracker(t.metainfo.Announce)
	for _, tier := range t.metainfo.AnnounceList {
		for _, url := range tier {
			t.startScrapingTracker(url)
		}
	}
}

// Returns an AnnounceRequest with fields filled out to defaults and current
// values.
func (t *Torrent) announceRequest() tracker.AnnounceRequest {
	// Note that IPAddress is not set. It's set for UDP inside the tracker
	// code, since it's dependent on the network in use.
	return tracker.AnnounceRequest{
		Event:    tracker.None,
		NumWant:  -1,
		Port:     uint16(t.cl.incomingPeerPort()),
		PeerId:   t.cl.peerID,
		InfoHash: t.infoHash,
		Key:      t.cl.announceKey(),

		// The following are vaguely described in BEP 3.

		Left:     t.bytesLeftAnnounce(),
		Uploaded: t.stats.BytesWrittenData.Int64(),
		// There's no mention of wasted or unwanted download in the BEP.
		Downloaded: t.stats.BytesReadUsefulData.Int64(),
	}
}

// Adds peers revealed in an announce until the announce ends, or we have
// enough peers.
func (t *Torrent) consumeDHTAnnounce(pvs <-chan dht.PeersValues) {
	cl := t.cl
	// Count all the unique addresses we got during this announce.
	allAddrs := make(map[string]struct{})
	for {
		select {
		case v, ok := <-pvs:
			if !ok {
				return
			}
			addPeers := make([]Peer, 0, len(v.Peers))
			for _, cp := range v.Peers {
				if cp.Port == 0 {
					// Can't do anything with this.
					continue
				}
				addPeers = append(addPeers, Peer{
					IP:     cp.IP[:],
					Port:   cp.Port,
					Source: peerSourceDHTGetPeers,
				})
				key := (&net.UDPAddr{
					IP:   cp.IP[:],
					Port: cp.Port,
				}).String()
				allAddrs[key] = struct{}{}
			}
			cl.lock()
			t.addPeers(addPeers)
			numPeers := t.peers.Len()
			cl.unlock()
			if numPeers >= cl.config.TorrentPeersHighWater {
				return
			}
		case <-t.closed.LockedChan(cl.locker()):
			return
		}
	}
}

func (t *Torrent) announceDHT(impliedPort bool, s *dht.Server) (err error) {
	cl := t.cl
	ps, err := s.Announce(t.infoHash, cl.incomingPeerPort(), impliedPort)
	if err != nil {
		return
	}
	t.consumeDHTAnnounce(ps.Peers)
	ps.Close()
	return
}

func (t *Torrent) dhtAnnouncer(s *dht.Server) {
	cl := t.cl
	for {
		select {
		case <-t.wantPeersEvent.LockedChan(cl.locker()):
		case <-t.closed.LockedChan(cl.locker()):
			return
		}
		err := t.announceDHT(true, s)
		func() {
			cl.lock()
			defer cl.unlock()
			if err == nil {
				t.numDHTAnnounces++
			} else {
				log.Printf("error announcing %q to DHT: %s", t, err)
			}
		}()
		select {
		case <-t.closed.LockedChan(cl.locker()):
			return
		case <-time.After(5 * time.Minute):
		}
	}
}

func (t *Torrent) addPeers(peers []Peer) {
	for _, p := range peers {
		t.addPeer(p)
	}
}

func (t *Torrent) Stats() TorrentStats {
	t.cl.rLock()
	defer t.cl.rUnlock()
	return t.statsLocked()
}

func (t *Torrent) statsLocked() (ret TorrentStats) {
	ret.ActivePeers = len(t.conns)
	ret.HalfOpenPeers = len(t.halfOpen)
	ret.PendingPeers = t.peers.Len()
	ret.TotalPeers = t.numTotalPeers()
	ret.ConnectedSeeders = 0
	for c := range t.conns {
		if all, ok := c.peerHasAllPieces(); all && ok {
			ret.ConnectedSeeders++
		}
	}
	ret.ConnStats = t.stats.Copy()
	return
}

// The total number of peers in the torrent.
func (t *Torrent) numTotalPeers() int {
	peers := make(map[string]struct{})
	for conn := range t.conns {
		ra := conn.conn.RemoteAddr()
		if ra == nil {
			// It's been closed and doesn't support RemoteAddr.
			continue
		}
		peers[ra.String()] = struct{}{}
	}
	for addr := range t.halfOpen {
		peers[addr] = struct{}{}
	}
	t.peers.Each(func(peer Peer) {
		peers[fmt.Sprintf("%s:%d", peer.IP, peer.Port)] = struct{}{}
	})
	return len(peers)
}

// Reconcile bytes transferred before connection was associated with a
// torrent.
func (t *Torrent) reconcileHandshakeStats(c *connection) {
	if c.stats != (ConnStats{
		// Handshakes should only increment these fields:
		BytesWritten: c.stats.BytesWritten,
		BytesRead:    c.stats.BytesRead,
	}) {
		panic("bad stats")
	}
	c.postHandshakeStats(func(cs *ConnStats) {
		cs.BytesRead.Add(c.stats.BytesRead.Int64())
		cs.BytesWritten.Add(c.stats.BytesWritten.Int64())
	})
	c.reconciledHandshakeStats = true
}

// Returns true if the connection is added.
func (t *Torrent) addConnection(c *connection) (err error) {
	defer func() {
		if err == nil {
			torrent.Add("added connections", 1)
		}
	}()
	if t.closed.IsSet() {
		return errors.New("torrent closed")
	}
	for c0 := range t.conns {
		if c.PeerID != c0.PeerID {
			continue
		}
		if !t.cl.config.dropDuplicatePeerIds {
			continue
		}
		if left, ok := c.hasPreferredNetworkOver(c0); ok && left {
			c0.Close()
			t.deleteConnection(c0)
		} else {
			return errors.New("existing connection preferred")
		}
	}
	if len(t.conns) >= t.maxEstablishedConns {
		c := t.worstBadConn()
		if c == nil {
			return errors.New("don't want conns")
		}
		c.Close()
		t.deleteConnection(c)
	}
	if len(t.conns) >= t.maxEstablishedConns {
		panic(len(t.conns))
	}
	t.conns[c] = struct{}{}
	return nil
}

func (t *Torrent) wantConns() bool {
	if !t.networkingEnabled {
		return false
	}
	if t.closed.IsSet() {
		return false
	}
	if !t.seeding() && !t.needData() {
		return false
	}
	if len(t.conns) < t.maxEstablishedConns {
		return true
	}
	return t.worstBadConn() != nil
}

func (t *Torrent) SetMaxEstablishedConns(max int) (oldMax int) {
	t.cl.lock()
	defer t.cl.unlock()
	oldMax = t.maxEstablishedConns
	t.maxEstablishedConns = max
	wcs := slices.HeapInterface(slices.FromMapKeys(t.conns), worseConn)
	for len(t.conns) > t.maxEstablishedConns && wcs.Len() > 0 {
		t.dropConnection(wcs.Pop().(*connection))
	}
	t.openNewConns()
	return oldMax
}

func (t *Torrent) pieceHashed(piece pieceIndex, correct bool) {
	log.Fmsg("hashed piece %d", piece).Add("piece", piece).Add("passed", correct).AddValue(debugLogValue).Log(t.logger)
	if t.closed.IsSet() {
		return
	}
	p := &t.pieces[piece]
	touchers := t.reapPieceTouchers(piece)
	if p.storageCompletionOk {
		// Don't score the first time a piece is hashed, it could be an
		// initial check.
		if correct {
			pieceHashedCorrect.Add(1)
		} else {
			log.Printf("%s: piece %d (%s) failed hash: %d connections contributed", t, piece, p.hash, len(touchers))
			pieceHashedNotCorrect.Add(1)
		}
	}
	if correct {
		if len(touchers) != 0 {
			// Don't increment stats above connection-level for every involved
			// connection.
			t.allStats((*ConnStats).incrementPiecesDirtiedGood)
		}
		for _, c := range touchers {
			c.stats.incrementPiecesDirtiedGood()
		}
		err := p.Storage().MarkComplete()
		if err != nil {
			log.Printf("%T: error marking piece complete %d: %s", t.storage, piece, err)
		}
	} else {
		if len(touchers) != 0 {
			// Don't increment stats above connection-level for every involved
			// connection.
			t.allStats((*ConnStats).incrementPiecesDirtiedBad)
			for _, c := range touchers {
				// Y u do dis peer?!
				c.stats.incrementPiecesDirtiedBad()
			}
			slices.Sort(touchers, connLessTrusted)
			if t.cl.config.Debug {
				log.Printf("dropping first corresponding conn from trust: %v", func() (ret []int64) {
					for _, c := range touchers {
						ret = append(ret, c.netGoodPiecesDirtied())
					}
					return
				}())
			}
			c := touchers[0]
			t.cl.banPeerIP(c.remoteAddr.IP)
			c.Drop()
		}
		t.onIncompletePiece(piece)
		p.Storage().MarkNotComplete()
	}
	t.updatePieceCompletion(piece)
}

func (t *Torrent) cancelRequestsForPiece(piece pieceIndex) {
	// TODO: Make faster
	for cn := range t.conns {
		cn.tickleWriter()
	}
}

func (t *Torrent) onPieceCompleted(piece pieceIndex) {
	t.pendAllChunkSpecs(piece)
	t.cancelRequestsForPiece(piece)
	for conn := range t.conns {
		conn.Have(piece)
	}
}

// Called when a piece is found to be not complete.
func (t *Torrent) onIncompletePiece(piece pieceIndex) {
	if t.pieceAllDirty(piece) {
		t.pendAllChunkSpecs(piece)
	}
	if !t.wantPieceIndex(piece) {
		// log.Printf("piece %d incomplete and unwanted", piece)
		return
	}
	// We could drop any connections that we told we have a piece that we
	// don't here. But there's a test failure, and it seems clients don't care
	// if you request pieces that you already claim to have. Pruning bad
	// connections might just remove any connections that aren't treating us
	// favourably anyway.

	// for c := range t.conns {
	// 	if c.sentHave(piece) {
	// 		c.Drop()
	// 	}
	// }
	for conn := range t.conns {
		if conn.PeerHasPiece(piece) {
			conn.updateRequests()
		}
	}
}

func (t *Torrent) verifyPiece(piece pieceIndex) {
	cl := t.cl
	cl.lock()
	defer cl.unlock()
	p := &t.pieces[piece]
	defer func() {
		p.numVerifies++
		cl.event.Broadcast()
	}()
	for p.hashing || t.storage == nil {
		cl.event.Wait()
	}
	if !p.t.piecesQueuedForHash.Remove(bitmap.BitIndex(piece)) {
		panic("piece was not queued")
	}
	t.updatePiecePriority(piece)
	if t.closed.IsSet() || t.pieceComplete(piece) {
		return
	}
	p.hashing = true
	t.publishPieceChange(piece)
	t.updatePiecePriority(piece)
	t.storageLock.RLock()
	cl.unlock()
	sum := t.hashPiece(piece)
	t.storageLock.RUnlock()
	cl.lock()
	p.hashing = false
	t.updatePiecePriority(piece)
	t.pieceHashed(piece, sum == p.hash)
	t.publishPieceChange(piece)
}

// Return the connections that touched a piece, and clear the entries while
// doing it.
func (t *Torrent) reapPieceTouchers(piece pieceIndex) (ret []*connection) {
	for c := range t.pieces[piece].dirtiers {
		delete(c.peerTouchedPieces, piece)
		ret = append(ret, c)
	}
	t.pieces[piece].dirtiers = nil
	return
}

func (t *Torrent) connsAsSlice() (ret []*connection) {
	for c := range t.conns {
		ret = append(ret, c)
	}
	return
}

// Currently doesn't really queue, but should in the future.
func (t *Torrent) queuePieceCheck(pieceIndex pieceIndex) {
	piece := &t.pieces[pieceIndex]
	if piece.queuedForHash() {
		return
	}
	t.piecesQueuedForHash.Add(bitmap.BitIndex(pieceIndex))
	t.publishPieceChange(pieceIndex)
	t.updatePiecePriority(pieceIndex)
	go t.verifyPiece(pieceIndex)
}

func (t *Torrent) VerifyData() {
	for i := pieceIndex(0); i < t.NumPieces(); i++ {
		t.Piece(i).VerifyData()
	}
}

// Start the process of connecting to the given peer for the given torrent if
// appropriate.
func (t *Torrent) initiateConn(peer Peer) {
	if peer.Id == t.cl.peerID {
		return
	}
	if t.cl.badPeerIPPort(peer.IP, peer.Port) {
		return
	}
	addr := IpPort{peer.IP, uint16(peer.Port)}
	if t.addrActive(addr.String()) {
		return
	}
	t.halfOpen[addr.String()] = peer
	go t.cl.outgoingConnection(t, addr, peer.Source)
}

func (t *Torrent) AddClientPeer(cl *Client) {
	t.AddPeers(func() (ps []Peer) {
		for _, la := range cl.ListenAddrs() {
			ps = append(ps, Peer{
				IP:   missinggo.AddrIP(la),
				Port: missinggo.AddrPort(la),
			})
		}
		return
	}())
}

// All stats that include this Torrent. Useful when we want to increment
// ConnStats but not for every connection.
func (t *Torrent) allStats(f func(*ConnStats)) {
	f(&t.stats)
	f(&t.cl.stats)
}

func (t *Torrent) hashingPiece(i pieceIndex) bool {
	return t.pieces[i].hashing
}

func (t *Torrent) pieceQueuedForHash(i pieceIndex) bool {
	return t.piecesQueuedForHash.Get(bitmap.BitIndex(i))
}

func (t *Torrent) dialTimeout() time.Duration {
	return reducedDialTimeout(t.cl.config.MinDialTimeout, t.cl.config.NominalDialTimeout, t.cl.config.HalfOpenConnsPerTorrent, t.peers.Len())
}
