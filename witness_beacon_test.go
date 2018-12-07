package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/extpreimage"
	"github.com/lightningnetwork/lnd/htlcswitch"
)

// mockInvoice implements the channeldb.InvoiceTerm
// interface, allowing us to stub it out for testing
// LookupPreimage in isolation.
type mockInvoice struct {
	channeldb.InvoiceTerm

	expectedTimeLock      uint32
	expectedCurrentHeight uint32
	expectedClient        extpreimage.Client
	expectedRegistry      channeldb.InvoiceRegistry
	preimage              [32]byte
	tempErr               error
	permErr               error
}

// GetPaymentPreimage stubs channeldb.InvoiceTerm.GetPaymentPreimage by
// checking the passed parameters against those expected in the tests,
// and returning any stubbed errors or preimages.
func (i *mockInvoice) GetPaymentPreimage(timeLock uint32,
	currentHeight uint32, client extpreimage.Client,
	registry channeldb.InvoiceRegistry) (
	[32]byte, channeldb.TempPreimageError, channeldb.PermPreimageError) {
	var zeroPreimage [32]byte

	if i.tempErr != nil {
		return zeroPreimage, i.tempErr, nil
	}

	if i.permErr != nil {
		return zeroPreimage, nil, i.permErr
	}

	if i.expectedTimeLock != timeLock {
		return zeroPreimage, nil, fmt.Errorf("Wrong timeLock: expected %v, "+
			"got %v", i.expectedTimeLock, timeLock)
	}

	if i.expectedCurrentHeight != currentHeight {
		return zeroPreimage, nil, fmt.Errorf("Wrong currentHeight: expected %v, "+
			"got %v", i.expectedCurrentHeight, currentHeight)
	}

	if i.expectedClient != client {
		return zeroPreimage, nil, fmt.Errorf("Wrong client: expected %v, "+
			"got %v", i.expectedClient, client)
	}

	if i.expectedRegistry != registry {
		return zeroPreimage, nil, fmt.Errorf("Wrong registry: expected %v, "+
			"got %v", i.expectedRegistry, registry)
	}

	return i.preimage, nil, nil
}

// mockRegistry implements the htlcswitch.InvoiceDatabase interface
// and allows us to use a map to store invoices for the purposes of testing.
type mockRegistry struct {
	htlcswitch.InvoiceDatabase

	invoices map[chainhash.Hash]*channeldb.Invoice
}

// LookupInvoice returns from our mock invoice registry
func (r *mockRegistry) LookupInvoice(rHash chainhash.Hash) (channeldb.Invoice,
	uint32, error) {
	if r.invoices[rHash] != nil {
		return *r.invoices[rHash], 0, nil
	}

	return channeldb.Invoice{}, 0, fmt.Errorf("unable to find invoice for "+
		"hash %v", rHash)
}

// mockExtpreimageClient implements extpreimage.Client to check that we
// are passing the expected client.
type mockExtpreimageClient struct {
	extpreimage.Client
}

func TestLookupPreimage(t *testing.T) {
	var preimage [32]byte
	_, err := rand.Read(preimage[:])
	if err != nil {
		t.Fatalf("Unable to create preimage: %v", err)
	}

	hash := sha256.Sum256(preimage[:])

	client := &mockExtpreimageClient{}
	registry := &mockRegistry{
		invoices: make(map[chainhash.Hash]*channeldb.Invoice),
	}

	invoice := &channeldb.Invoice{
		Memo: hash[:],
	}

	var invoiceKey chainhash.Hash
	copy(invoiceKey[:], hash[:])
	registry.invoices[invoiceKey] = invoice

	tests := []struct {
		name            string
		payHash         []byte
		expectedInvoice channeldb.Invoice
		invoice         *mockInvoice
		preimage        []byte
		hasPreimage     bool
	}{
		{
			name:            "preimage is returned",
			payHash:         hash[:],
			expectedInvoice: *invoice,
			invoice: &mockInvoice{
				expectedTimeLock:      uint32(0),
				expectedCurrentHeight: uint32(0),
				expectedClient:        client,
				expectedRegistry:      registry,
				preimage:              preimage,
			},
			preimage:    preimage[:],
			hasPreimage: true,
		},
		{
			name:            "temp error is returned",
			payHash:         hash[:],
			expectedInvoice: *invoice,
			invoice: &mockInvoice{
				tempErr: fmt.Errorf("fake temp error"),
			},
			preimage:    nil,
			hasPreimage: false,
		},
		{
			name:            "perm error is returned",
			payHash:         hash[:],
			expectedInvoice: *invoice,
			invoice: &mockInvoice{
				permErr: fmt.Errorf("fake perm error"),
			},
			preimage:    nil,
			hasPreimage: false,
		},
	}

	for _, test := range tests {

		// stub castInvoiceTerm so that we can use our mockInvoice, which
		// implements the channeldb.InvoiceTerm interface
		revert := castInvoiceTerm
		defer func() { castInvoiceTerm = revert }()
		castInvoiceTerm = func(i channeldb.Invoice) channeldb.InvoiceTerm {
			if !bytes.Equal(i.Memo, test.expectedInvoice.Memo) {
				t.Fatalf("cast to channeldb.InvoiceTerm failed, "+
					"expected: %v, got %v", test.expectedInvoice, i)
			}
			return test.invoice
		}

		p := &preimageBeacon{
			invoices:          registry,
			extpreimageClient: client,
		}

		preimage, ok := p.LookupPreimage(test.payHash)

		if ok != test.hasPreimage {
			t.Errorf("LookupPreimage test \"%s\" failed, got ok: %t, "+
				"want ok: %t", test.name, ok, test.hasPreimage)
		}

		if !bytes.Equal(preimage[:], test.preimage[:]) {
			t.Errorf("LookupPreimage test \"%s\" failed, got preimage: %v, "+
				"want preimage: %v", test.name, preimage, test.preimage)
		}
	}
}
