package extpreimage

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/lnwire"
)

// Invoice is a simplified view of a channeldb.Invoice that includes
// the information needed to lookup an external preimage. We do
// this to move as much logic out out of the main LND packages as
// possible.
type Invoice struct {
	// ExternalPreimage indicates if the preimage for this hash is
	// stored externally and must be retrieved.
	ExternalPreimage bool

	// PaymentHash is the hash that locks the HTLC for this payment.
	PaymentHash [sha256.Size]byte

	// PaymentPreimage is the preimage which is to be revealed in the
	// occasion that an HTLC paying to the hash of this preimage is
	// extended.
	PaymentPreimage [32]byte

	// Value is the expected amount of milli-satoshis to be paid to an HTLC
	// which can be satisfied by the above preimage.
	Value lnwire.MilliSatoshi

	// Settled indicates if this particular contract term has been fully
	// settled by the payer.
	Settled bool
}

// InvoiceRegistry is a registry for storing invoices. It mirrors
// the structure of main.InvoiceRegistry.
type InvoiceRegistry interface {
	AddInvoicePreimage(chainhash.Hash, [32]byte) error
}

// GetPaymentHash retrieves the payment hash for a given invoice,
// either by calculating it from the preimage, or using the given
// hash for invoices with external preimages.
func (i *Invoice) GetPaymentHash() ([32]byte, error) {
	var zeroHash [32]byte
	var paymentHash [32]byte
	var zeroPreimage [32]byte

	if i.ExternalPreimage {
		if bytes.Equal(i.PaymentHash[:], zeroHash[:]) {
			return zeroHash, fmt.Errorf("Invoices with ExternalPreimage must " +
				"have a locally defined PaymentHash.")
		}

		// For external preimages, we rely on a provided hash
		paymentHash = i.PaymentHash
	} else {
		if bytes.Equal(i.PaymentPreimage[:], zeroPreimage[:]) {
			return zeroHash, fmt.Errorf("Invoices must have a preimage or" +
				"use ExternalPreimages")
		}

		// For local preimages, we calculate the hash ourselves
		paymentHash = sha256.Sum256(i.PaymentPreimage[:])
	}

	return paymentHash, nil
}

// GetPaymentPreimage retrieves the preimage for a given invoice,
// either by pulling it directly from the invoice, or by retrieving
// it from the external preimage service if it is an external preimage
// invoice.
func (i *Invoice) GetPaymentPreimage(timeLock uint32, currentHeight uint32,
	extpreimageClient Client, registry InvoiceRegistry) ([32]byte, error, error) {
	var zeroPreimage [32]byte

	switch {
	// if there is a local preimage available, we should use it to settle the
	// invoice
	case !bytes.Equal(i.PaymentPreimage[:], zeroPreimage[:]):
		return i.PaymentPreimage, nil, nil
	// if this is an invoice with an external preimage, we should retrieve it.
	case i.ExternalPreimage:
		if extpreimageClient == nil {
			return zeroPreimage, fmt.Errorf("no extpreimage client configured"), nil
		}

		preimageRequest := &PreimageRequest{
			PaymentHash: i.PaymentHash,
			Amount:      int64(i.Value.ToSatoshis()),
			TimeLock:    timeLock,
			BestHeight:  currentHeight,
		}

		preimage, tempErr, permErr := extpreimageClient.Retrieve(preimageRequest)

		if permErr != nil {
			return zeroPreimage, nil, permErr
		}

		if tempErr != nil {
			return zeroPreimage, tempErr, nil
		}

		// we should persist the preimage locally before settling the
		// invoice so that it can be used as a normal invoice once it
		// is settled, and used for any duplicate payments without
		// making another request to the external preimage service.
		invoiceHash := chainhash.Hash(i.PaymentHash)
		err := registry.AddInvoicePreimage(invoiceHash, preimage)

		if err != nil {
			return zeroPreimage, err, nil
		}

		return preimage, nil, nil
	}

	return zeroPreimage, nil, fmt.Errorf("no preimage available on invoice")
}
