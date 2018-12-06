package main

import (
	"sync"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/contractcourt"
	"github.com/lightningnetwork/lnd/extpreimage"
	"github.com/lightningnetwork/lnd/htlcswitch"
	"github.com/lightningnetwork/lnd/lnwallet"
)

// preimageSubscriber reprints an active subscription to be notified once the
// daemon discovers new preimages, either on chain or off-chain.
type preimageSubscriber struct {
	updateChan chan []byte

	quit chan struct{}
}

// witnessCache is an interface describing the subset of channeldb.WitnessCache
// used by the witness beacon.
type witnessCache interface {
	LookupWitness(channeldb.WitnessType, []byte) ([]byte, error)
	AddWitness(channeldb.WitnessType, []byte) error
}

// preimageBeacon is an implementation of the contractcourt.WitnessBeacon
// interface, and the lnwallet.PreimageCache interface. This implementation is
// concerned with a single witness type: sha256 hahsh preimages.
type preimageBeacon struct {
	sync.RWMutex

	extpreimageClient extpreimage.Client

	invoices htlcswitch.InvoiceDatabase

	wCache witnessCache

	clientCounter uint64
	subscribers   map[uint64]*preimageSubscriber

	pollCounter uint64
	polls       map[uint64](chan bool)
}

// Stop shuts down the preimage beacon by quitting
// all currently active polls.
func (p *preimageBeacon) Stop() {
	if p.polls == nil {
		p.polls = make(map[uint64](chan bool))
	}
	for _, quit := range p.polls {
		quit <- true
	}
	p.polls = make(map[uint64](chan bool))
}

// SubscribeUpdates returns a channel that will be sent upon *each* time a new
// preimage is discovered.
func (p *preimageBeacon) SubscribeUpdates() *contractcourt.WitnessSubscription {
	p.Lock()
	defer p.Unlock()

	clientID := p.clientCounter
	client := &preimageSubscriber{
		updateChan: make(chan []byte, 10),
		quit:       make(chan struct{}),
	}

	p.subscribers[p.clientCounter] = client

	p.clientCounter++

	srvrLog.Debugf("Creating new witness beacon subscriber, id=%v",
		p.clientCounter)

	return &contractcourt.WitnessSubscription{
		WitnessUpdates: client.updateChan,
		CancelSubscription: func() {
			p.Lock()
			defer p.Unlock()

			delete(p.subscribers, clientID)

			close(client.quit)
		},
	}
}

// castInvoiceTerm converts a standard Invoice into one that
// can be used for retrieving preimages. We do this purely so that
// we can stub invoices for testing.
var castInvoiceTerm = func(i channeldb.Invoice) channeldb.InvoiceTerm {
	invoiceTerm := i.Terms
	return &invoiceTerm
}

// LookupPreImage attempts to lookup a preimage in the global cache.  True is
// returned for the second argument if the preimage is found.
func (p *preimageBeacon) LookupPreimage(payHash []byte) ([]byte, bool) {
	p.RLock()
	defer p.RUnlock()

	// First, we'll check the invoice registry to see if we already know of
	// the preimage as it's on that we created ourselves.
	var invoiceKey chainhash.Hash
	copy(invoiceKey[:], payHash)
	invoice, _, err := p.invoices.LookupInvoice(invoiceKey)
	switch {
	case err == channeldb.ErrInvoiceNotFound:
		// If we get this error, then it simply means that this invoice
		// wasn't found, so we don't treat it as a critical error.
	case err != nil:
		return nil, false
	}

	// If we've found the invoice, then we can return the preimage
	// directly.
	if err != channeldb.ErrInvoiceNotFound {
		// We get the preimage from the invoice, either using a local preimage
		// if it's available,  or an external preimage if it's not. Note that
		// we are using zero values for both the HTLC expiry and current block height:
		// this is because we care only about external preimages that are readily
		// available, not those that need to be requested further.
		invoiceTerm := castInvoiceTerm(invoice)
		preimage, tempErr, permErr := invoiceTerm.GetPaymentPreimage(
			uint32(0), uint32(0), p.extpreimageClient, p.invoices)

		if permErr != nil {
			ltndLog.Errorf("permanent error while retrieving invoice "+
				"preimage: %v", permErr)
			return nil, false
		}

		if tempErr != nil {
			ltndLog.Errorf("temporary error while retrieving invoice "+
				"preimage: %v", tempErr)
			ltndLog.Debugf("initiating polling to retrieve preimage "+
				"for payment hash: %v", payHash)

			go p.pollExtpreimage(invoiceTerm, 10*time.Second)
			return nil, false
		}

		return preimage[:], true
	}

	// Otherwise, we'll perform a final check using the witness cache.
	preimage, err := p.wCache.LookupWitness(
		channeldb.Sha256HashWitness, payHash,
	)
	if err != nil {
		ltndLog.Errorf("unable to lookup witness: %v", err)
		return nil, false
	}

	return preimage, true
}

func (p *preimageBeacon) pollExtpreimage(i channeldb.InvoiceTerm,
	pollInterval time.Duration) {
	// create a channel for us to be able to stop polling if the
	// preimage beacon is quit.
	if p.polls == nil {
		p.polls = make(map[uint64](chan bool))
	}
	pollID := p.pollCounter
	quit := make(chan bool)
	p.polls[pollID] = quit
	p.pollCounter++
	defer func() {
		delete(p.polls, pollID)
		close(quit)
	}()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			preimage, tempErr, permErr := i.GetPaymentPreimage(uint32(0), uint32(0),
				p.extpreimageClient, p.invoices)

			// permanent errors indicate we no longer need to poll for this
			// preimage, we'll never retrieve it.
			if permErr != nil {
				ltndLog.Errorf("permanent error while polling for invoice "+
					"preimage: %v, stopping poll.", permErr)
				return
			}

			// if we encountered another temporary error, just continue to poll.
			if tempErr != nil {
				ltndLog.Errorf("temporary error while polling for invoice "+
					"preimage: %v, continuing poll.", tempErr)
				continue
			}

			// if we encountered no permanent or temporary errors, we have our
			// preimage and we can add it to the cache and stop polling.
			ltndLog.Debugf("retrieved preimage=%x while polling.", preimage[:])
			p.AddPreimage(preimage[:])
			return
		case <-quit:
			ltndLog.Tracef("preimage Beacon shutting down, stopping poll.")
			return
		}
	}
}

// AddPreImage adds a newly discovered preimage to the global cache, and also
// signals any subscribers of the newly discovered witness.
func (p *preimageBeacon) AddPreimage(pre []byte) error {
	p.Lock()
	defer p.Unlock()

	srvrLog.Infof("Adding preimage=%x to witness cache", pre[:])

	// First, we'll add the witness to the decaying witness cache.
	err := p.wCache.AddWitness(channeldb.Sha256HashWitness, pre)
	if err != nil {
		return err
	}

	// With the preimage added to our state, we'll now send a new
	// notification to all subscribers.
	for _, client := range p.subscribers {
		go func(c *preimageSubscriber) {
			select {
			case c.updateChan <- pre:
			case <-c.quit:
				return
			}
		}(client)
	}

	return nil
}

var _ contractcourt.WitnessBeacon = (*preimageBeacon)(nil)
var _ lnwallet.PreimageCache = (*preimageBeacon)(nil)
