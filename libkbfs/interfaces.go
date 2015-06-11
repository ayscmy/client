package libkbfs

import (
	"fmt"
	"time"

	"github.com/keybase/client/go/libkb"
	keybase1 "github.com/keybase/client/protocol/go"
)

// Block just needs to be (de)serialized using msgpack
type Block interface{}

// BlockContext is used by the server to help identify blocks
type BlockContext interface {
	// GetWriter returns the UID of the writer for the corresponding block
	GetWriter() keybase1.UID
	// GetQuotaSize returns the encrypted size of the corresponding
	// block (i.e., how much the writer will be charged for this
	// block).
	GetQuotaSize() uint32
}

// KBFSOps handles all file system operations.  Expands all indirect
// pointers.  Operations that modify the server data change all the
// block IDs along the path, and so must return a path with the new
// BlockIds so the caller can update their references.
//
// KBFSOps implementations must guarantee goroutine-safety of calls on
// a per-top-level-folder basis.
//
// There are two types of operations that could block:
//   * remote-sync operations, that need to synchronously update the
//     MD for the corresponding top-level folder.  When these
//     operations return successfully, they will have guaranteed to
//     have successfully written the modification to the KBFS servers.
//   * remote-access operations, that don't sync any modifications to KBFS
//     servers, but may block on reading data from the servers.
//
// KBFSOps implementations are supposed to give git-like consistency
// semantics for modification operations; they will be visible to
// other clients immediately after the remote-sync operations succeed,
// if and only if there was no other intervening modification to the
// same folder.  If not, the change will be sync'd to the server in a
// special per-device "unmerged" area before the operation succeeds.
// In this case, the modification will not be visible to other clients
// until the KBFS code on this device performs automatic conflict
// resolution in the background.
type KBFSOps interface {
	// GetFavDirs returns the logged-in user's list of favorite
	// top-level folders.  This is a remote-access operation.
	GetFavDirs() ([]DirID, error)
	// GetOrCreateRootPathByHandle returns the root path, and root
	// directory entry associated with the given DirHandle, if the
	// logged-in user has read permissions to the top-level folder.
	// It creates the folder if one doesn't exist yet, and the
	// logged-in user has write permissions to the top-level folder.
	// This is a remote-access operation.
	GetOrCreateRootPathForHandle(handle *DirHandle) (Path, DirEntry, error)
	// GetRootPath returns the root path, root directory entry, and
	// handle associated with the given DirID, if the logged-in user
	// has read permissions to the top-level folder.  This is a
	// remote-access operation.
	GetRootPath(dir DirID) (Path, DirEntry, *DirHandle, error)
	// GetDir returns the directory block (including a complete list
	// of all the children in that directory and their metadata), if
	// the logged-in user has read permission for the top-level
	// folder.  This is a remote-access operation.
	GetDir(dir Path) (*DirBlock, error)
	// CreateDir creates a new subdirectory under the given path, if
	// the logged-in user has write permission to the top-level
	// folder.  Returns the new Path for the created subdirectory, and
	// its new directory entry.  This is a remote-sync operation.
	CreateDir(dir Path, path string) (Path, DirEntry, error)
	// CreateFile creates a new file under the given path, if the
	// logged-in user has write permission to the top-level folder.
	// Returns the new Path for the created file, and its new
	// directory entry.  This is a remote-sync operation.
	CreateFile(dir Path, path string, isEx bool) (Path, DirEntry, error)
	// CreateLink creates a new symlink under the given path, if the
	// logged-in user has write permission to the top-level folder.
	// Returns the new Path for the created symlink, and its new
	// directory entry.  This is a remote-sync operation.
	CreateLink(dir Path, fromPath string, toPath string) (
		Path, DirEntry, error)
	// RemoveDir removes the subdirectory represented by the given
	// path, if the logged-in user has write permission to the
	// top-level folder.  Will return an error if the subdirectory is
	// not empty.  Returns the new Path for the parent directory.
	// This is a remote-sync operation.
	RemoveDir(dir Path) (Path, error)
	// RemoveEntry removes the directory entry represented by the
	// given path, if the logged-in user has write permission to the
	// top-level folder.  Returns the new Path for the parent
	// directory.  This is a remote-sync operation.
	RemoveEntry(file Path) (Path, error)
	// Rename performs an atomic rename operation with a given
	// top-level folder if the logged-in user has write permission to
	// that folder, and will return an error if paths from different
	// folders are passed in.  Returns an updated path for the old
	// parent directory, and an updated path for the new directory
	// entry.  This is a remote-sync operation.
	Rename(oldParent Path, oldName string, newParent Path, newName string) (
		Path, Path, error)
	// Read fills in the given buffer with data from the file at the
	// given path starting at the given offset, if the logged-in user
	// has read permission to the top-level folder.  The read data
	// reflects any outstanding writes and truncates to that file that
	// have been written through this KBFSOps object, even if those
	// writes have not yet been sync'd.  There is no guarantee that
	// Read returns all of the requested data; it will return the
	// number of bytes that it wrote to the dest buffer.  Reads on an
	// unlinked file may or may not succeed, depending on whether or
	// not the data has been cached locally.  This is a remote-access
	// operation.
	Read(file Path, dest []byte, off int64) (int64, error)
	// Write modifies the file at the given path, by writing the given
	// buffer at the given offset within the file, if the logged-in
	// user has write permission to the top-level folder.  It
	// overwrites any data already there, and extends the file size as
	// necessary to accomodate the new data.  It guarantees to write
	// the entire buffer in one operation.  Writes on an unlinked file
	// may or may not succeed as no-ops, depending on whether or not
	// the necessary blocks have been locally cached.  This is a
	// remote-access operation.
	Write(file Path, data []byte, off int64) error
	// Truncate modifies the file at the given path, by either
	// shrinking or extending its size to match the given size, if the
	// logged-in user has write permission to the top-level folder.
	// If extending the file, it pads the new data with 0s.  Truncates
	// on an unlinked file may or may not succeed as no-ops, depending
	// on whether or not the necessary blocks have been locally
	// cached.  This is a remote-access operation.
	Truncate(file Path, size uint64) error
	// SetEx turns on or off the executable bit on the file
	// represented by a given path, if the logged-in user has write
	// permissions to the top-level folder.  It returns the updated
	// path to the file.  This is a remote-sync operation.
	SetEx(file Path, ex bool) (newPath Path, err error)
	// SetMtime sets the modification time on the file represented by
	// a given path, if the logged-in user has write permissions to
	// the top-level folder.  It returns the updated path to the
	// file. If mtime is nil, it is a noop.  This is a remote-sync
	// operation.
	SetMtime(file Path, mtime *time.Time) (Path, error)
	// Sync flushes all outstanding writes and truncates for the given
	// file to the KBFS servers, if the logged-in user has write
	// permissions to the top-level folder.  If done through a file
	// system interface, this may include modifications done via
	// multiple file handles.  It returns the updated path to the
	// file.  This is a remote-sync operation.
	Sync(file Path) (Path, error)
}

// KBPKI interacts with kbpkid to fetch info from keybase
type KBPKI interface {
	// ResolveAssertion loads a user by assertion (could also be a username)
	ResolveAssertion(input string) (*libkb.User, error)
	// GetUser loads user by UID and checks assumptions via identify
	GetUser(uid keybase1.UID) (*libkb.User, error)
	// GetSession gets the current keybase session
	GetSession() (*libkb.Session, error)
	// GetLoggedInUser gets the UID of the current logged-in user
	GetLoggedInUser() (keybase1.UID, error)
	// HasVerifyingKey returns nil if the given user has the given
	// VerifyingKey, and an error otherwise.
	//
	// TODO: Add a timestamp argument (or similar) so that we can
	// check for revocation.
	HasVerifyingKey(uid keybase1.UID, verifyingKey VerifyingKey) error
	// GetCryptPublicKeys gets all of a user's crypt public keys (one
	// per device).
	GetCryptPublicKeys(uid keybase1.UID) ([]CryptPublicKey, error)
	// GetCurrentCryptPublicKey gets the crypt public key for the
	// currently-active device.
	GetCurrentCryptPublicKey() (CryptPublicKey, error)
}

// KeyManager fetches and constructs the keys needed for KBFS file
// operations.
type KeyManager interface {
	// GetTLFCryptKeyForEncryption gets the crypt key to use for
	// encryption (i.e., with the latest key generation) for the
	// TLF with the given metadata.
	GetTLFCryptKeyForEncryption(md *RootMetadata) (TLFCryptKey, error)

	// GetTLFCryptKeyForMDDecryption gets the crypt key to use for
	// the TLF with the given metadata to decrypt the private
	// portion of the metadata.
	GetTLFCryptKeyForMDDecryption(md *RootMetadata) (TLFCryptKey, error)

	// GetTLFCryptKeyForBlockDecryption gets the crypt key to use
	// for the TLF with the given metadata to decrypt the block
	// pointed to by the given pointer.
	GetTLFCryptKeyForBlockDecryption(md *RootMetadata, blockPtr BlockPointer) (TLFCryptKey, error)

	// Rekey creates a new epoch of keys for the given directory
	Rekey(md *RootMetadata) error
}

// ReportingLevel indicate the severity of a reported event.
// MK: I sort of have something like this with G.Log.Debug, G.Log.Warn, etc..
// JS: Yeah, I was thinking about that, but I was a bit unsure about tying it
//     to a purely string-based logging system, in case we want to report
//     more complex objects.  Not sure if this is the right way to go though.
//     Very open to suggestions.
// MK: Yeah, very good point...
type ReportingLevel int

const (
	// RptD indicates a debug-level event
	RptD ReportingLevel = iota
	// RptI indicates a info-level event
	RptI
	// RptW indicates a warning-level event
	RptW
	// RptE indicates a error-level event
	RptE
	// RptF indicates a fatal-level event
	RptF
)

// Reporter exports events (asynchronously) to any number of sinks
type Reporter interface {
	// Report records that a given event happened at the given reporting level.
	Report(level ReportingLevel, message fmt.Stringer)
	// LastError returns the last error-level event that occurred on
	// this device.
	LastError() (string, *time.Time)
}

// MDCache gets and puts plaintext top-level metadata into the cache.
type MDCache interface {
	// Get gets the metadata object associated with the given MD ID.
	Get(id MdID) (*RootMetadata, error)
	// Put stores the metadata object associated with the given MD ID.
	Put(id MdID, md *RootMetadata) error
}

// KeyCache handles caching for both TLFCryptKeys and BlockCryptKeys.
type KeyCache interface {
	// GetTLFCryptKey gets the crypt key for the given TLF.
	GetTLFCryptKey(DirID, KeyGen) (TLFCryptKey, error)
	// PutTLFCryptKey stores the crypt key for the given TLF.
	PutTLFCryptKey(DirID, KeyGen, TLFCryptKey) error
}

// BlockCache gets and puts plaintext dir blocks and file blocks into
// a cache.
type BlockCache interface {
	// Get gets the block associated with the given block ID.  Returns
	// the dirty block for the given ID, if one exists.
	Get(id BlockID) (Block, error)
	// Put stores the block associated with the given block ID, and
	// marks it as dirty if it has outstanding changes beyond the
	// "official" version of the block with that ID.
	Put(id BlockID, block Block, dirty bool) error
	// Delete removes the block associated with the given block ID
	// from the cache.  No error is returned if no block exists
	// for the given ID.
	Delete(id BlockID) error
	// Finalize transitions a dirty block, stored under the block's
	// old block ID, to a new block with the new ID.
	Finalize(oldID BlockID, newID BlockID) error
	// IsDirty states whether or not the given block ID is dirty in
	// this cache.
	IsDirty(id BlockID) bool
}

// Crypto signs, verifies, encrypts, and decrypts stuff.
type Crypto interface {
	// MakeRandomDirID generates a dir ID using a CSPRNG.
	MakeRandomDirID(isPublic bool) (DirID, error)

	// MakeTemporaryBlockID generates a temporary block ID using a
	// CSPRNG. This is used for indirect blocks before they're
	// committed to the server.
	MakeTemporaryBlockID() (BlockID, error)

	// MakeRandomTLFKeys generates top-level folder keys using a CSPRNG.
	MakeRandomTLFKeys() (TLFPublicKey, TLFPrivateKey, TLFEphemeralPublicKey,
		TLFEphemeralPrivateKey, TLFCryptKey, error)
	// MakeRandomTLFCryptKeyServerHalf generates the server-side of a
	// top-level folder crypt key.
	MakeRandomTLFCryptKeyServerHalf() (TLFCryptKeyServerHalf, error)
	// MakeRandomBlockCryptKeyServerHalf generates the server-side of
	// a block crypt key.
	MakeRandomBlockCryptKeyServerHalf() (BlockCryptKeyServerHalf, error)

	// MaskTLFCryptKey returns the client-side of a top-level folder crypt key.
	MaskTLFCryptKey(serverHalf TLFCryptKeyServerHalf, key TLFCryptKey) (
		TLFCryptKeyClientHalf, error)
	// UnmaskTLFCryptKey returns the top-level folder crypt key.
	UnmaskTLFCryptKey(serverHalf TLFCryptKeyServerHalf,
		clientHalf TLFCryptKeyClientHalf) (TLFCryptKey, error)
	// UnmaskBlockCryptKey returns the block crypt key.
	UnmaskBlockCryptKey(serverHalf BlockCryptKeyServerHalf,
		tlfCryptKey TLFCryptKey) (BlockCryptKey, error)

	// Sign signs the msg with the current device's private key.
	Sign(msg []byte) (sigInfo SignatureInfo, err error)
	// Verify verifies that sig matches msg being signed with the
	// private key that corresponds to verifyingKey.
	Verify(msg []byte, sigInfo SignatureInfo) error

	// EncryptTLFCryptKeyClientHalf encrypts a TLFCryptKeyClientHalf
	// using both a TLF's ephemeral private key and a device pubkey.
	EncryptTLFCryptKeyClientHalf(privateKey TLFEphemeralPrivateKey,
		publicKey CryptPublicKey, clientHalf TLFCryptKeyClientHalf) (
		EncryptedTLFCryptKeyClientHalf, error)

	// DecryptTLFCryptKeyClientHalf decrypts a TLFCryptKeyClientHalf
	// using the current device's private key and the TLF's ephemeral
	// public key.
	DecryptTLFCryptKeyClientHalf(publicKey TLFEphemeralPublicKey,
		encryptedClientHalf EncryptedTLFCryptKeyClientHalf) (
		TLFCryptKeyClientHalf, error)

	// EncryptPrivateMetadata encrypts a serialized PrivateMetadata object.
	EncryptPrivateMetadata(buf []byte, key TLFCryptKey) ([]byte, error)
	// DecryptPrivateMetadata decrypts a serialized PrivateMetadata object.
	DecryptPrivateMetadata(buf []byte, key TLFCryptKey) ([]byte, error)

	// EncryptBlocks encrypts a block. plainSize is the size of the encoded
	// block; EncryptBlock() must guarantee that plainSize <=
	// len(encryptedBlock).
	EncryptBlock(block Block, key BlockCryptKey) (
		plainSize int, encryptedBlock []byte, err error)

	// DecryptBlock decrypts a block. Similar to EncryptBlock(),
	// DecryptBlock() must guarantee that (size of the decrypted
	// block) <= len(encryptedBlock).
	DecryptBlock(encryptedBlock []byte, key BlockCryptKey, block Block) error

	// Mac computes a keyed MAC of buf using a shared secret derived
	// from the given MacPublicKey and the current user's MAC private
	// key.
	MAC(publicKey MacPublicKey, buf []byte) (MAC, error)
	// VerifyMac verifies a given key and buf would hash to the given
	// mac.  The mac should indicate its type.
	VerifyMAC(publicKey MacPublicKey, buf []byte, mac MAC) error

	// Hash computes a deterministic hash of buf.
	Hash(buf []byte) (libkb.NodeHash, error)
	// VerifyHash verifies a given hash (the hash should include its
	// type).
	VerifyHash(buf []byte, hash libkb.NodeHash) error
}

// Codec encodes and decodes arbitrary data
type Codec interface {
	// Decode unmarshals the given buffer into the given object, if possible.
	Decode(buf []byte, obj interface{}) error
	// Encode marshals the given object into a returned buffer.
	Encode(obj interface{}) ([]byte, error)
}

// MDOps gets and puts root metadata to an MDServer.  On a get, it
// verifies the metadata is signed by the metadata's signing key.
type MDOps interface {
	// GetAtHandle returns the current metadata object corresponding
	// to the given top-level folder's handle, if the logged-in user
	// has read permission on the folder.  It creates the folder if
	// one doesn't exist yet, and the logged-in user has write
	// permissions to the top-level folder.
	GetAtHandle(handle *DirHandle) (*RootMetadata, error)
	// Get returns the current metadata object corresponding to the
	// given top-level folder, if the logged-in user has read
	// permission on the folder.
	Get(id DirID) (*RootMetadata, error)
	// GetAtID returns the metadata object corresponding to the given
	// top-level folder's handle, which matches the provided MD ID, if
	// one exists and the logged-in user has read permissions on the
	// folder.
	GetAtID(id DirID, mdID MdID) (*RootMetadata, error)
	// Put stores the given metadata object for the top-level folder
	// on the server, if the logged-in user has write permission on
	// the folder.
	Put(id DirID, md *RootMetadata) error
	// GetFavorites returns the logged-in user's list of favorite
	// top-level folders.
	GetFavorites() ([]DirID, error)
}

// KeyOps fetches server-side key halves and MAC public keys from the
// key server.
type KeyOps interface {
	// GetTLFCryptKeyServerHalf gets the server-side key half for a
	// device (identified by its CryptPublicKey) for a given TLF.
	GetTLFCryptKeyServerHalf(id DirID, keyGen KeyGen,
		cryptPublicKey CryptPublicKey) (TLFCryptKeyServerHalf, error)
	// PutTLFCryptKeyServerHalf puts the server-side key half for a
	// device (identified by its CryptPublicKey) for a given TLF.
	PutTLFCryptKeyServerHalf(id DirID, keyGen KeyGen,
		cryptPublicKey CryptPublicKey, serverHalf TLFCryptKeyServerHalf) error

	// GetMacPublicKey gets the public MAC key for a given user.
	GetMacPublicKey(uid keybase1.UID) (MacPublicKey, error)
}

// BlockOps gets and puts data blocks to a BlockServer. It performs
// the necessary crypto operations on each block.
type BlockOps interface {
	// Get gets the block associated with the given block ID and
	// context, uses the block key generated from the given TLF key
	// and the fetched server-side block key-half to decrypt, and
	// fills in the provided block object with its contents, if the
	// logged-in user has read permission for that block.
	Get(id BlockID, context BlockContext, tlfCryptKey TLFCryptKey,
		block Block) error
	// Ready turns plaintext blocks into encrypted buffers using the
	// provided key, and calculates their IDs and sizes, so that we
	// can do a bunch of block puts in parallel for every write.
	// Ready() must guarantee that plainSize <= len(buf).
	Ready(block Block, cryptKey BlockCryptKey) (
		id BlockID, plainSize int, buf []byte, err error)
	// Put stores the (encrypted) block data under the given ID and
	// context on the server, along with the server half of the block key.
	Put(id BlockID, tlfID DirID, context BlockContext, buf []byte,
		serverHalf BlockCryptKeyServerHalf) error
	// Delete instructs the server to delete the block data associated
	// with the given ID and context.
	Delete(id BlockID, context BlockContext) error
}

// MDServer gets and puts metadata for each top-level directory.  The
// instantiation should be able to fetch session/user details via
// KBPKI.  On a put, the server is responsible for 1) ensuring the
// user has write permissions; 2) ensuring the writer appears as
// LastWriter; 3) ensuring the LastWriter matches the current session;
// and 4) detecting conflicting writes based on the previous root
// block ID (i.e., when it supports strict consistency).  On a get, it
// verifies the logged-in user has read permissions.
//
// TODO: Add interface for searching by time
// TODO: PutFavorites() to allow for signed favorites list
type MDServer interface {
	// GetAtHandle returns the current (signed/encrypted) metadata
	// object corresponding to the given top-level folder's handle, if
	// the logged-in user has read permission on the folder.  It
	// creates the folder if one doesn't exist yet, and the logged-in
	// user has permission to do so.
	GetAtHandle(handle *DirHandle) (*RootMetadataSigned, error)
	// Get returns the current (signed/encrypted) metadata object
	// corresponding to the given top-level folder, if the logged-in
	// user has read permission on the folder.
	Get(id DirID) (*RootMetadataSigned, error)
	// GetAtID returns the (signed/encrypted) metadata object
	// corresponding to the given top-level folder's handle, which
	// matches the provided MD ID, if one exists and the logged-in
	// user has read permission on the folder.
	GetAtID(id DirID, mdID MdID) (*RootMetadataSigned, error)
	// Put stores the (signed/encrypted) metadata object for the given
	// top-level folder, under the given MD ID.
	Put(id DirID, mdID MdID, md *RootMetadataSigned) error
	// GetFavorites returns the logged-in user's list of favorite
	// top-level folders.
	// TODO: this data should be at least signed.
	GetFavorites() ([]DirID, error)
}

// BlockServer gets and puts opaque data blocks.  The instantiation
// should be able to fetch session/user details via KBPKI.  On a
// put/delete, the server is reponsible for: 1) checking that the ID
// matches the hash of the buffer; and 2) enforcing writer quotas.
type BlockServer interface {
	// Get gets the (encrypted) block data associated with the given
	// block ID and context, uses the provided block key to decrypt
	// the block, and fills in the provided block object with its
	// contents, if the logged-in user has read permission for that
	// block.
	Get(id BlockID, context BlockContext) (
		[]byte, BlockCryptKeyServerHalf, error)
	// Put stores the (encrypted) block data under the given ID and
	// context on the server, along with the server half of the block
	// key.
	Put(id BlockID, tlfID DirID, context BlockContext, buf []byte,
		serverHalf BlockCryptKeyServerHalf) error
	// Delete instructs the server to delete the block data
	// associated with the given ID. No error is returned if no
	// data exists for the given ID.
	Delete(id BlockID, context BlockContext) error
}

// BlockSplitter decides when a file or directory block needs to be split
type BlockSplitter interface {
	// CopyUntilSplit copies data into the block until we reach the
	// point where we should split, but only if writing to the end of
	// the last block.  If this is writing into the middle of a file,
	// just copy everything that will fit into the block, and assume
	// that block boundaries will be fixed later. Return how much was
	// copied.
	CopyUntilSplit(
		block *FileBlock, lastBlock bool, data []byte, off int64) int64

	// CheckSplit, given a block, figures out whether it ends at the
	// right place.  If so, return 0.  If not, return either the
	// offset in the block where it should be split, or -1 if more
	// bytes from the next block should be appended.
	CheckSplit(block *FileBlock) int64

	// ShouldEmbedBlockChanges decides whether we should keep the
	// block changes embedded in the MD or not.
	ShouldEmbedBlockChanges(bc *BlockChanges) bool
}

// Observer can be notified that there is an available update for a
// given directory.  The notification callbacks should not block, or
// make any calls to the Notifier interface.
type Observer interface {
	// LocalChange announces that the file at this path has been
	// updated locally, but not yet saved at the server.  The nodes
	// along the path are still identified by the same IDs.
	LocalChange(path Path)
	// BatchChanges announces that the files at this path have all
	// been updated together, and may have changed their IDs.
	BatchChanges(dir DirID, paths []Path)
	// TODO: Notify about changes in favorites list
}

// Notifier notifies registrants of directory changes
type Notifier interface {
	// RegisterForChanges declares that the given Observer wants to
	// subscribe to updates for the given top-level folders.
	RegisterForChanges(dirs []DirID, obs Observer) error
	// UnregisterFromChanges declares that the given Observer no
	// longer wants to subscribe to updates for the given top-level
	// folders.
	UnregisterFromChanges(dirs []DirID, obs Observer) error
}

// Config collects all the singleton instance instantiations needed to
// run KBFS in one place.  The methods below are self-explanatory and
// do not require comments.
type Config interface {
	KBFSOps() KBFSOps
	SetKBFSOps(KBFSOps)
	KBPKI() KBPKI
	SetKBPKI(KBPKI)
	KeyManager() KeyManager
	SetKeyManager(KeyManager)
	Reporter() Reporter
	SetReporter(Reporter)
	MDCache() MDCache
	SetMDCache(MDCache)
	KeyCache() KeyCache
	SetKeyCache(KeyCache)
	BlockCache() BlockCache
	SetBlockCache(BlockCache)
	Crypto() Crypto
	SetCrypto(Crypto)
	Codec() Codec
	SetCodec(Codec)
	MDOps() MDOps
	SetMDOps(MDOps)
	KeyOps() KeyOps
	SetKeyOps(KeyOps)
	BlockOps() BlockOps
	SetBlockOps(BlockOps)
	MDServer() MDServer
	SetMDServer(MDServer)
	BlockServer() BlockServer
	SetBlockServer(BlockServer)
	BlockSplitter() BlockSplitter
	SetBlockSplitter(BlockSplitter)
	Notifier() Notifier
	SetNotifier(Notifier)
	DataVersion() DataVer
	// ReqsBufSize indicates the number of read or write operations
	// that can be buffered per folder
	ReqsBufSize() int
}
