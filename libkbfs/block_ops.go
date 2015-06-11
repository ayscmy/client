package libkbfs

import (
	"github.com/keybase/client/go/libkb"
)

// BlockOpsStandard implements the BlockOps interface by relaying
// requests to the block server.
type BlockOpsStandard struct {
	config Config
}

var _ BlockOps = (*BlockOpsStandard)(nil)

// Get implements the BlockOps interface for BlockOpsStandard.
func (b *BlockOpsStandard) Get(id BlockID, context BlockContext,
	tlfCryptKey TLFCryptKey, block Block) error {
	bserv := b.config.BlockServer()
	buf, blockServerHalf, err := bserv.Get(id, context)
	if err != nil {
		return err
	}
	if context.GetQuotaSize() != uint32(len(buf)) {
		err = &InconsistentByteCountError{
			ExpectedByteCount: int(context.GetQuotaSize()),
			ByteCount:         len(buf),
		}
		return err
	}

	// construct the block crypt key
	blockCryptKey, err := b.config.Crypto().UnmaskBlockCryptKey(
		blockServerHalf, tlfCryptKey)
	if err != nil {
		return err
	}

	// decrypt the block
	return b.config.Crypto().DecryptBlock(buf, blockCryptKey, block)
}

// Ready implements the BlockOps interface for BlockOpsStandard.
func (b *BlockOpsStandard) Ready(
	block Block, cryptKey BlockCryptKey) (id BlockID, plainSize int, buf []byte, err error) {
	defer func() {
		if err != nil {
			id = BlockID{}
			plainSize = 0
			buf = nil
		}
	}()
	crypto := b.config.Crypto()
	if plainSize, buf, err = crypto.EncryptBlock(block, cryptKey); err != nil {
		return
	}

	if len(buf) < plainSize {
		err = &TooLowByteCountError{
			ExpectedMinByteCount: plainSize,
			ByteCount:            len(buf),
		}
		return
	}

	// now get the block ID for the buffer
	var h libkb.NodeHash
	if h, err = crypto.Hash(buf); err != nil {
		return
	}

	var nhs libkb.NodeHashShort
	var ok bool
	if nhs, ok = h.(libkb.NodeHashShort); !ok {
		err = &BadCryptoError{id}
		return
	}

	id = BlockID(nhs)
	return
}

// Put implements the BlockOps interface for BlockOpsStandard.
func (b *BlockOpsStandard) Put(id BlockID, tlfID DirID, context BlockContext,
	buf []byte, serverHalf BlockCryptKeyServerHalf) (err error) {
	if context.GetQuotaSize() != uint32(len(buf)) {
		err = &InconsistentByteCountError{
			ExpectedByteCount: int(context.GetQuotaSize()),
			ByteCount:         len(buf),
		}
		return
	}
	bserv := b.config.BlockServer()
	err = bserv.Put(id, tlfID, context, buf, serverHalf)
	return
}

// Delete implements the BlockOps interface for BlockOpsStandard.
func (b *BlockOpsStandard) Delete(id BlockID, context BlockContext) error {
	bserv := b.config.BlockServer()
	err := bserv.Delete(id, context)
	return err
}
