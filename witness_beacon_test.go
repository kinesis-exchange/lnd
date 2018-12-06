package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"os"
	"os/signal"
	"testing"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/extpreimage"
	"github.com/lightningnetwork/lnd/htlcswitch"
)

func shutdownBeacon(p *preimageBeacon) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt)

	go func() {
		select {
		case sig := <-c:
			fmt.Printf("Got %s signal. Aborting...\n", sig)
			p.Stop()
		}
	}()
}

// mockInvoice implements the channeldb.InvoiceTerm
// interface, allowing us to stub it out for testing
// LookupPreimage in isolation.
type mockInvoice struct {
	channeldb.InvoiceTerm
	expectedTimeLock      uint32
	expectedCurrentHeight uint32
	expectedClient        extpreimage.Client
	expectedRegistry      channeldb.InvoiceRegistry
	requestCount          uint64
	responses             []*mockPreimageResponse
}

// mockPreimageResponse stores a response to
// GetPaymentPreimage in order
type mockPreimageResponse struct {
	preimage [32]byte
	tempErr  error
	permErr  error
}

// GetPaymentPreimage stubs channeldb.InvoiceTerm.GetPaymentPreimage by
// checking the passed parameters against those expected in the tests,
// and returning any stubbed errors or preimages.
func (i *mockInvoice) GetPaymentPreimage(timeLock uint32,
	currentHeight uint32, client extpreimage.Client,
	registry channeldb.InvoiceRegistry) (
	[32]byte, channeldb.TempPreimageError, channeldb.PermPreimageError) {
	var zeroPreimage [32]byte

	// get the next response to return and increment
	// so the following request gets another response
	r := i.responses[i.requestCount]
	i.requestCount++

	if r.tempErr != nil {
		return zeroPreimage, r.tempErr, nil
	}

	if r.permErr != nil {
		return zeroPreimage, nil, r.permErr
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

	return r.preimage, nil, nil
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

// mockWitnessCache implements the witnessCache interface to check that
// we are adding preimages to the cache during polling.
type mockWitnessCache struct {
	witnessCache
	witnesses [][]byte
}

func (w *mockWitnessCache) AddWitness(wType channeldb.WitnessType,
	witness []byte) error {
	w.witnesses = append(w.witnesses, witness)

	return nil
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
	wCache := &mockWitnessCache{
		witnesses: make([][]byte, 0),
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
		andCheck        func() error
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
				responses: []*mockPreimageResponse{
					&mockPreimageResponse{
						preimage: preimage,
					},
				},
			},
			preimage:    preimage[:],
			hasPreimage: true,
		},
		{
			name:            "temp error is returned",
			payHash:         hash[:],
			expectedInvoice: *invoice,
			invoice: &mockInvoice{
				responses: []*mockPreimageResponse{
					&mockPreimageResponse{
						tempErr: fmt.Errorf("fake temp error"),
					},
					// we return a permanent error to stop polling
					&mockPreimageResponse{
						permErr: fmt.Errorf("fake perm error"),
					},
				},
			},
			preimage:    nil,
			hasPreimage: false,
		},
		{
			name:            "perm error is returned",
			payHash:         hash[:],
			expectedInvoice: *invoice,
			invoice: &mockInvoice{
				responses: []*mockPreimageResponse{
					&mockPreimageResponse{
						permErr: fmt.Errorf("fake perm error"),
					},
				},
			},
			preimage:    nil,
			hasPreimage: false,
		},
		{
			name:            "preimage is returned after temp error",
			payHash:         hash[:],
			expectedInvoice: *invoice,
			invoice: &mockInvoice{
				expectedTimeLock:      uint32(0),
				expectedCurrentHeight: uint32(0),
				expectedClient:        client,
				expectedRegistry:      registry,
				responses: []*mockPreimageResponse{
					&mockPreimageResponse{
						tempErr: fmt.Errorf("fake temp error"),
					},
					&mockPreimageResponse{
						preimage: preimage,
					},
				},
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
			wCache:            wCache,
			invoices:          registry,
			extpreimageClient: client,
		}

		// Shut down our preimage beacon if the process stops
		shutdownBeacon(p)

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

func TestPollExtpreimage(t *testing.T) {
	var preimage [32]byte
	_, err := rand.Read(preimage[:])
	if err != nil {
		t.Fatalf("Unable to create preimage: %v", err)
	}
	client := &mockExtpreimageClient{}
	registry := &mockRegistry{}

	tests := []struct {
		name        string
		invoice     *mockInvoice
		wCache      *mockWitnessCache
		wCacheLen   int
		wCacheIndex int
		wCacheValue []byte
	}{
		{
			name: "temp error, then preimage",
			invoice: &mockInvoice{
				expectedTimeLock:      uint32(0),
				expectedCurrentHeight: uint32(0),
				expectedClient:        client,
				expectedRegistry:      registry,
				responses: []*mockPreimageResponse{
					&mockPreimageResponse{
						tempErr: fmt.Errorf("fake temp error"),
					},
					&mockPreimageResponse{
						preimage: preimage,
					},
				},
			},
			wCache: &mockWitnessCache{
				witnesses: make([][]byte, 0),
			},
			wCacheLen:   1,
			wCacheIndex: 0,
			wCacheValue: preimage[:],
		},
		{
			name: "temp error, then perm error",
			invoice: &mockInvoice{
				responses: []*mockPreimageResponse{
					&mockPreimageResponse{
						tempErr: fmt.Errorf("fake temp error"),
					},
					&mockPreimageResponse{
						permErr: fmt.Errorf("fake perm error"),
					},
				},
			},
			wCache: &mockWitnessCache{
				witnesses: make([][]byte, 0),
			},
			wCacheLen: 0,
		},
	}

	for _, test := range tests {
		p := &preimageBeacon{
			wCache:            test.wCache,
			invoices:          registry,
			extpreimageClient: client,
		}

		// Shut down our preimage beacon if the process stops
		shutdownBeacon(p)

		p.pollExtpreimage(test.invoice, 1*time.Millisecond)

		time.Sleep(2 * time.Millisecond)

		if len(test.wCache.witnesses) != test.wCacheLen {
			t.Errorf("Wrong size for witness cache: "+
				"expected %d, got %d", test.wCacheLen, len(test.wCache.witnesses))
		}
		if len(test.wCache.witnesses) != 0 &&
			!bytes.Equal(test.wCache.witnesses[test.wCacheIndex],
				test.wCacheValue[:]) {
			t.Errorf("Wrong wCacheValue in cache: expected %v, got %v",
				test.wCacheValue[:], test.wCache.witnesses[test.wCacheIndex])
		}
	}
}
