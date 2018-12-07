package channeldb

import (
  "bytes"
  "crypto/sha256"
  "fmt"

  "github.com/btcsuite/btcd/chaincfg/chainhash"
  "github.com/lightningnetwork/lnd/extpreimage"
)

// InvoiceRegistry is a registry for storing invoices. It is a
// simplified interface of htlcswitch.InvoiceDatabase for use
// in this package.
type InvoiceRegistry interface {
  AddInvoicePreimage(chainhash.Hash, [32]byte) error
}

// tempPreimageError is an error encountered while retrieving
// a preimage which is temporary - we may be able to eventually
// recover the preimage, but it is in an unknown state.
type tempPreimageError interface {
  Error() string
}

// permPreimageError is an error encountered while retrieving
// a preimage which is permanent - we should never expect to recover
// the preimage.
type permPreimageError interface {
  Error() string
}


// GetPaymentHash retrieves the payment hash for a given invoice,
// either by calculating it from the preimage, or using the given
// hash for invoices with external preimages.
func (c *ContractTerm) GetPaymentHash() ([32]byte, error) {
  var zeroHash [32]byte
  var paymentHash [32]byte
  var zeroPreimage [32]byte

  if c.ExternalPreimage {
    if bytes.Equal(c.PaymentHash[:], zeroHash[:]) {
      return zeroHash, fmt.Errorf("Invoices with ExternalPreimage must " +
        "have a locally defined PaymentHash.")
    }

    // For external preimages, we rely on a provided hash
    paymentHash = c.PaymentHash
  } else {
    if bytes.Equal(c.PaymentPreimage[:], zeroPreimage[:]) {
      return zeroHash, fmt.Errorf("Invoices must have a preimage or" +
        "use ExternalPreimages")
    }

    // For local preimages, we calculate the hash ourselves
    paymentHash = sha256.Sum256(c.PaymentPreimage[:])
  }

  return paymentHash, nil
}

// GetPaymentPreimage retrieves the preimage for a given invoice,
// either by pulling it directly from the invoice, or by retrieving
// it from the external preimage service if it is an external preimage
// invoice.
func (c *ContractTerm) GetPaymentPreimage(timeLock uint32, currentHeight uint32,
  client extpreimage.Client, registry InvoiceRegistry) (
    [32]byte, tempPreimageError, permPreimageError) {

  var zeroPreimage [32]byte

  switch {
  // if there is a local preimage available, we should use it to settle the
  // invoice
  case !bytes.Equal(c.PaymentPreimage[:], zeroPreimage[:]):
    return c.PaymentPreimage, nil, nil
  // if this is an invoice with an external preimage, we should retrieve it.
  case c.ExternalPreimage:
    if client == nil {
      return zeroPreimage, fmt.Errorf("no extpreimage client configured"), nil
    }

    preimageRequest := &extpreimage.PreimageRequest{
      PaymentHash: c.PaymentHash,
      Amount:      int64(c.Value.ToSatoshis()),
      TimeLock:    timeLock,
      BestHeight:  currentHeight,
    }

    preimage, tempErr, permErr := client.Retrieve(preimageRequest)

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
    invoiceHash := chainhash.Hash(c.PaymentHash)
    err := registry.AddInvoicePreimage(invoiceHash, preimage)

    if err != nil {
      return zeroPreimage, err, nil
    }

    return preimage, nil, nil
  }

  return zeroPreimage, nil, fmt.Errorf("no preimage available on invoice")
}
