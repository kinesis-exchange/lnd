package extpreimage_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/extpreimage"
)

type mockExtpreimageClient struct {
	extpreimage.Client
	expectedRequest *extpreimage.PreimageRequest
	preimage        [32]byte
	tempErr         error
	permErr         error
}

func (c *mockExtpreimageClient) Retrieve(req *extpreimage.PreimageRequest) (
	[32]byte, error, error) {
	var zeroPreimage [32]byte

	// if no expectation is set, just return whatever was passed
	if c.expectedRequest == nil {
		return c.preimage, c.tempErr, c.permErr
	}

	if !bytes.Equal(c.expectedRequest.PaymentHash[:], req.PaymentHash[:]) {
		return zeroPreimage, nil, fmt.Errorf("Wrong PaymentHash: expected %v, "+
			"got %v", c.expectedRequest.PaymentHash, req.PaymentHash)
	}

	if c.expectedRequest.Amount != req.Amount {
		return zeroPreimage, nil, fmt.Errorf("Wrong Amount: expected %v, "+
			"got %v", c.expectedRequest.Amount, req.Amount)
	}

	if c.expectedRequest.TimeLock != req.TimeLock {
		return zeroPreimage, nil, fmt.Errorf("Wrong TimeLock: expected %v, "+
			"got %v", c.expectedRequest.TimeLock, req.TimeLock)
	}

	if c.expectedRequest.BestHeight != req.BestHeight {
		return zeroPreimage, nil, fmt.Errorf("Wrong BestHeight: expected %v, "+
			"got %v", c.expectedRequest.BestHeight, req.BestHeight)
	}

	return c.preimage, c.tempErr, c.permErr
}

func (c *mockExtpreimageClient) Stop() error {
	return nil
}

type mockRegistry struct {
	expectedHash     chainhash.Hash
	expectedPreimage [32]byte
	err              error
}

func (r *mockRegistry) AddInvoicePreimage(hash chainhash.Hash,
	preimage [32]byte) error {
	if r.err != nil {
		return r.err
	}

	if !bytes.Equal(r.expectedHash[:], hash[:]) {
		return fmt.Errorf("Wrong hash: expected %v, got %v",
			r.expectedHash, hash)
	}

	if !bytes.Equal(r.expectedPreimage[:], preimage[:]) {
		return fmt.Errorf("Wrong preimage: expected %v, got %v",
			r.expectedPreimage, preimage)
	}

	return nil
}

func TestGetPaymentHash(t *testing.T) {
	var zeroPreimage [32]byte
	var zeroHash [sha256.Size]byte

	var preimage [32]byte
	_, err := rand.Read(preimage[:])
	if err != nil {
		t.Fatalf("Unable to create preimage: %v", err)
	}

	hash := sha256.Sum256(preimage[:])

	tests := []struct {
		name    string
		invoice *extpreimage.Invoice
		hash    [sha256.Size]byte
		err     error
	}{
		// if it is an external preimage, use the local hash
		{
			name: "external preimage with local hash",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: true,
				PaymentHash:      hash,
				PaymentPreimage:  zeroPreimage,
				Value:            1000,
				Settled:          false,
			},
			hash: hash,
			err:  nil,
		},
		// if it is an external preimage without a local hash, throw
		{
			name: "external preimage without local hash",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: true,
				PaymentHash:      zeroHash,
				PaymentPreimage:  zeroPreimage,
				Value:            1000,
				Settled:          false,
			},
			hash: zeroHash,
			err: fmt.Errorf("Invoices with ExternalPreimage must " +
				"have a locally defined PaymentHash."),
		},
		// if it is a local preimage without a preimage, throw
		{
			name: "local preimage without preimage",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: false,
				PaymentHash:      zeroHash,
				PaymentPreimage:  zeroPreimage,
				Value:            1000,
				Settled:          false,
			},
			hash: zeroHash,
			err: fmt.Errorf("Invoices must have a preimage or" +
				"use ExternalPreimages"),
		},
		// if it is a local preimage, calculate the hash
		{
			name: "local preimage with preimage",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: false,
				PaymentHash:      zeroHash,
				PaymentPreimage:  preimage,
				Value:            1000,
				Settled:          false,
			},
			hash: hash,
			err:  nil,
		},
	}

	for _, test := range tests {
		hash, err := test.invoice.GetPaymentHash()

		if (err == nil && test.err != nil) ||
			(err != nil && test.err == nil) ||
			(err != nil && test.err != nil && err.Error() != test.err.Error()) {
			t.Errorf("GetPaymentHash test \"%s\" failed, got err: %v, want err: %v",
				test.name, err, test.err)
		}

		if !bytes.Equal(hash[:], test.hash[:]) {
			t.Errorf("GetPaymentHash test \"%s\" failed, got hash: %v, "+
				"want hash: %v", test.name, hash, test.hash)
		}
	}
}

func TestGetPaymentPreimage(t *testing.T) {
	var zeroPreimage [32]byte
	var zeroHash [sha256.Size]byte

	var preimage [32]byte
	_, err := rand.Read(preimage[:])
	if err != nil {
		t.Fatalf("Unable to create preimage: %v", err)
	}

	hash := sha256.Sum256(preimage[:])

	timeLock := uint32(288)
	currentHeight := uint32(123456)
	registry := &mockRegistry{}

	tests := []struct {
		name              string
		invoice           *extpreimage.Invoice
		timeLock          uint32
		currentHeight     uint32
		extpreimageClient extpreimage.Client
		registry          extpreimage.InvoiceRegistry
		preimage          [32]byte
		tempErr           error
		permErr           error
	}{
		// if it has a preimage and is marked external, return it
		{
			name: "local preimage on external preimage invoice",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: true,
				PaymentHash:      hash,
				PaymentPreimage:  preimage,
				Value:            1000,
				Settled:          false,
			},
			timeLock:          timeLock,
			currentHeight:     currentHeight,
			extpreimageClient: &mockExtpreimageClient{},
			registry:          registry,
			preimage:          preimage,
			tempErr:           nil,
			permErr:           nil,
		},
		// if it has a preimage and is not marked external, return it
		{
			name: "local preimage on local preimage invoice",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: false,
				PaymentHash:      zeroHash,
				PaymentPreimage:  preimage,
				Value:            1000,
				Settled:          false,
			},
			timeLock:          timeLock,
			currentHeight:     currentHeight,
			extpreimageClient: &mockExtpreimageClient{},
			registry:          registry,
			preimage:          preimage,
			tempErr:           nil,
			permErr:           nil,
		},
		// if it is not external preimage, and does not have a preimage, throw
		{
			name: "no preimage on local preimage invoice",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: false,
				PaymentHash:      zeroHash,
				PaymentPreimage:  zeroPreimage,
				Value:            1000,
				Settled:          false,
			},
			timeLock:          timeLock,
			currentHeight:     currentHeight,
			extpreimageClient: &mockExtpreimageClient{},
			registry:          registry,
			preimage:          zeroPreimage,
			tempErr:           nil,
			permErr:           fmt.Errorf("no preimage available on invoice"),
		},
		// if it is an external preimage, and there is no client, throw
		{
			name: "no extpreimage client",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: true,
				PaymentHash:      hash,
				PaymentPreimage:  zeroPreimage,
				Value:            1000,
				Settled:          false,
			},
			timeLock:          timeLock,
			currentHeight:     currentHeight,
			extpreimageClient: nil,
			registry:          registry,
			preimage:          zeroPreimage,
			tempErr:           fmt.Errorf("no extpreimage client configured"),
			permErr:           nil,
		},
		// if it is an external preimage, return it
		{
			name: "external preimage retrieved",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: true,
				PaymentHash:      hash,
				PaymentPreimage:  zeroPreimage,
				Value:            1000,
				Settled:          false,
			},
			timeLock:      timeLock,
			currentHeight: currentHeight,
			extpreimageClient: &mockExtpreimageClient{
				expectedRequest: &extpreimage.PreimageRequest{
					PaymentHash: hash,
					// amounts are in satoshis, invoices are in millisatoshis
					Amount:     1,
					TimeLock:   timeLock,
					BestHeight: currentHeight,
				},
				preimage: preimage,
			},
			registry: &mockRegistry{
				expectedPreimage: preimage,
				expectedHash:     hash,
			},
			preimage: preimage,
			tempErr:  nil,
			permErr:  nil,
		},
		// if it encounters a permanent error, return that
		{
			name: "external preimage permanent error",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: true,
				PaymentHash:      hash,
				PaymentPreimage:  zeroPreimage,
				Value:            1000,
				Settled:          false,
			},
			timeLock:      timeLock,
			currentHeight: currentHeight,
			extpreimageClient: &mockExtpreimageClient{
				preimage: zeroPreimage,
				permErr:  fmt.Errorf("fake permanent error"),
			},
			registry: registry,
			preimage: zeroPreimage,
			tempErr:  nil,
			permErr:  fmt.Errorf("fake permanent error"),
		},
		// if it encounters a temporary error, return that
		{
			name: "external preimage temporary error",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: true,
				PaymentHash:      hash,
				PaymentPreimage:  zeroPreimage,
				Value:            1000,
				Settled:          false,
			},
			timeLock:      timeLock,
			currentHeight: currentHeight,
			extpreimageClient: &mockExtpreimageClient{
				preimage: zeroPreimage,
				tempErr:  fmt.Errorf("fake temporary error"),
			},
			registry: registry,
			preimage: zeroPreimage,
			tempErr:  fmt.Errorf("fake temporary error"),
			permErr:  nil,
		},
		// if it is an external preimage and can't be added to the registry, throw
		{
			name: "AddInvoicePreimage failure",
			invoice: &extpreimage.Invoice{
				ExternalPreimage: true,
				PaymentHash:      hash,
				PaymentPreimage:  zeroPreimage,
				Value:            1000,
				Settled:          false,
			},
			timeLock:      timeLock,
			currentHeight: currentHeight,
			extpreimageClient: &mockExtpreimageClient{
				preimage: preimage,
			},
			registry: &mockRegistry{
				err: fmt.Errorf("fake registry error"),
			},
			preimage: zeroPreimage,
			tempErr:  fmt.Errorf("fake registry error"),
			permErr:  nil,
		},
	}

	for _, test := range tests {
		preimage, tempErr, permErr := test.invoice.GetPaymentPreimage(test.timeLock,
			test.currentHeight, test.extpreimageClient, test.registry)

		if (tempErr == nil && test.tempErr != nil) ||
			(tempErr != nil && test.tempErr == nil) ||
			(tempErr != nil && test.tempErr != nil &&
				tempErr.Error() != test.tempErr.Error()) {
			t.Errorf("GetPaymentHash test \"%s\" failed, got tempErr: %v, want tempErr: %v",
				test.name, tempErr, test.tempErr)
		}

		if (permErr == nil && test.permErr != nil) ||
			(permErr != nil && test.permErr == nil) ||
			(permErr != nil && test.permErr != nil &&
				permErr.Error() != test.permErr.Error()) {
			t.Errorf("GetPaymentHash test \"%s\" failed, got permErr: %v, want permErr: %v",
				test.name, permErr, test.permErr)
		}

		if !bytes.Equal(preimage[:], test.preimage[:]) {
			t.Errorf("GetPaymentHash test \"%s\" failed, got preimage: %v, "+
				"want preimage: %v", test.name, preimage, test.preimage)
		}
	}
}
