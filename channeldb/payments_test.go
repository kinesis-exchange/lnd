package channeldb

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/lightningnetwork/lnd/lnwire"
)

var testTime time.Time

func stubTime() {
	// stub time.Now so that we can have a consistent time in our tests
	testTime = time.Unix(time.Now().Unix(), 0)
	now = func() time.Time { return testTime }
}

func unstubTime() {
	now = time.Now
}

func makeFakePayment() *OutgoingPayment {
	fakeInvoice := &Invoice{
		CreationDate:   testTime,
		Memo:           []byte(""),
		Receipt:        []byte(""),
		PaymentRequest: []byte(""),
	}

	fakeInvoice.Terms.Value = lnwire.NewMSatFromSatoshis(10000)

	fakePayment := &OutgoingPayment{
		Invoice: *fakeInvoice,
	}
	fakePayment.Path = make([][33]byte, 0)
	return fakePayment
}

func makeCompleteFakePayment() *OutgoingPayment {
	fakePayment := makeFakePayment()
	fakePaymentRoute := makeFakePaymentRoute()

	fakePayment.Fee = fakePaymentRoute.Fee
	fakePayment.Path = fakePaymentRoute.Path
	fakePayment.TimeLockLength = fakePaymentRoute.TimeLockLength

	copy(fakePayment.PaymentPreimage[:], rev[:])

	return fakePayment
}

func makeFakePaymentHash() [32]byte {
	var paymentHash [32]byte
	rBytes, _ := randomBytes(0, 32)
	copy(paymentHash[:], rBytes)

	return paymentHash
}

func makeFakePaymentRoute() *OutgoingPaymentRoute {
	fakePath := make([][33]byte, 3)
	for i := 0; i < 3; i++ {
		copy(fakePath[i][:], bytes.Repeat([]byte{byte(i)}, 33))
	}

	return &OutgoingPaymentRoute{
		Fee:            101,
		Path:           fakePath,
		TimeLockLength: 1000,
	}
}

// randomBytes creates random []byte with length in range [minLen, maxLen)
func randomBytes(minLen, maxLen int) ([]byte, error) {
	randBuf := make([]byte, minLen+rand.Intn(maxLen-minLen))

	if _, err := rand.Read(randBuf); err != nil {
		return nil, fmt.Errorf("Internal error. "+
			"Cannot generate random string: %v", err)
	}

	return randBuf, nil
}

func makeRandomFakePayment() (*OutgoingPayment, error) {
	fakeInvoice := &Invoice{
		// Use single second precision to avoid false positive test
		// failures due to the monotonic time component.
		CreationDate:   testTime,
		Memo:           []byte(""),
		Receipt:        []byte(""),
		PaymentRequest: []byte(""),
	}

	fakeInvoice.Terms.Value = lnwire.MilliSatoshi(rand.Intn(10000))

	fakePayment := &OutgoingPayment{
		Invoice: *fakeInvoice,
	}

	fakePayment.Path = make([][33]byte, 0)

	return fakePayment, nil
}

func makeRandomFakePaymentRoute() (*OutgoingPaymentRoute, error) {
	fakePathLen := 1 + rand.Intn(5)
	fakePath := make([][33]byte, fakePathLen)
	for i := 0; i < fakePathLen; i++ {
		b, err := randomBytes(33, 34)
		if err != nil {
			return nil, err
		}
		copy(fakePath[i][:], b)
	}

	return &OutgoingPaymentRoute{
		Fee:            lnwire.MilliSatoshi(rand.Intn(1001)),
		Path:           fakePath,
		TimeLockLength: uint32(rand.Intn(10000)),
	}, nil
}

func TestOutgoingPaymentSerialization(t *testing.T) {
	t.Parallel()

	fakePayment := makeCompleteFakePayment()

	var b bytes.Buffer
	if err := serializeOutgoingPayment(&b, fakePayment); err != nil {
		t.Fatalf("unable to serialize outgoing payment: %v", err)
	}

	newPayment, err := deserializeOutgoingPayment(&b)
	if err != nil {
		t.Fatalf("unable to deserialize outgoing payment: %v", err)
	}

	if !reflect.DeepEqual(fakePayment, newPayment) {
		t.Fatalf("Payments do not match after "+
			"serialization/deserialization %v vs %v",
			spew.Sdump(fakePayment),
			spew.Sdump(newPayment),
		)
	}
}

func TestAddPaymentWorkflow(t *testing.T) {
	t.Parallel()
	stubTime()
	defer unstubTime()

	db, cleanUp, err := makeTestDB()
	defer cleanUp()
	if err != nil {
		t.Fatalf("unable to make test db: %v", err)
	}

	fakePayment := makeFakePayment()
	fakePaymentHash := makeFakePaymentHash()
	err = db.AddPayment(fakePaymentHash, fakePayment.Invoice.Terms.Value)
	if err != nil {
		t.Fatalf("unable to put payment in DB: %v", err)
	}

	payments, err := db.FetchAllPayments()
	if err != nil {
		t.Fatalf("unable to fetch payments from DB: %v", err)
	}

	expectedPayments := []*OutgoingPayment{fakePayment}
	if !reflect.DeepEqual(payments, expectedPayments) {
		t.Fatalf("Wrong payments after reading from DB. "+
			"Got %v, want %v",
			spew.Sdump(payments),
			spew.Sdump(expectedPayments),
		)
	}

	// Make some random payments
	for i := 0; i < 5; i++ {
		randomPayment, err := makeRandomFakePayment()
		if err != nil {
			t.Fatalf("Internal error in tests: %v", err)
		}

		randomPaymentHash := makeFakePaymentHash()

		err = db.AddPayment(randomPaymentHash, randomPayment.Invoice.Terms.Value)
		if err != nil {
			t.Fatalf("unable to put payment in DB: %v", err)
		}

		expectedPayments = append(expectedPayments, randomPayment)
	}

	payments, err = db.FetchAllPayments()
	if err != nil {
		t.Fatalf("Can't get payments from DB: %v", err)
	}

	if !reflect.DeepEqual(payments, expectedPayments) {
		t.Fatalf("Wrong payments after reading from DB. "+
			"Got %v, want %v",
			spew.Sdump(payments),
			spew.Sdump(expectedPayments),
		)
	}

	// Delete all payments.
	if err = db.DeleteAllPayments(); err != nil {
		t.Fatalf("unable to delete payments from DB: %v", err)
	}

	// Check that there is no payments after deletion
	paymentsAfterDeletion, err := db.FetchAllPayments()
	if err != nil {
		t.Fatalf("Can't get payments after deletion: %v", err)
	}
	if len(paymentsAfterDeletion) != 0 {
		t.Fatalf("After deletion DB has %v payments, want %v",
			len(paymentsAfterDeletion), 0)
	}
}

func TestPaymentRouteWorkflow(t *testing.T) {
	t.Parallel()
	stubTime()
	defer unstubTime()

	db, cleanUp, err := makeTestDB()
	defer cleanUp()
	if err != nil {
		t.Fatalf("unable to make test db: %v", err)
	}

	fakePayment := makeFakePayment()
	fakePaymentHash := makeFakePaymentHash()
	err = db.AddPayment(fakePaymentHash, fakePayment.Invoice.Terms.Value)
	if err != nil {
		t.Fatalf("unable to put payment in DB: %v", err)
	}

	fakePaymentRoute := makeFakePaymentRoute()
	err = db.UpdatePaymentRoute(fakePaymentHash, fakePaymentRoute)
	if err != nil {
		t.Fatalf("unable to update payment route in DB: %v", err)
	}

	payments, err := db.FetchAllPayments()
	if err != nil {
		t.Fatalf("unable to fetch payments from DB: %v", err)
	}

	fakePayment.Path = fakePaymentRoute.Path
	fakePayment.TimeLockLength = fakePaymentRoute.TimeLockLength
	fakePayment.Fee = fakePaymentRoute.Fee

	expectedPayments := []*OutgoingPayment{fakePayment}
	if !reflect.DeepEqual(payments, expectedPayments) {
		t.Fatalf("Wrong payments after reading from DB. "+
			"Got %v, want %v",
			spew.Sdump(payments),
			spew.Sdump(expectedPayments),
		)
	}
}

func TestPaymentPreimageWorkflow(t *testing.T) {
	t.Parallel()
	stubTime()
	defer unstubTime()

	db, cleanUp, err := makeTestDB()
	defer cleanUp()
	if err != nil {
		t.Fatalf("unable to make test db: %v", err)
	}

	fakePayment := makeFakePayment()
	fakePaymentPreimage := makeFakePaymentHash()
	fakePaymentHash := sha256.Sum256(fakePaymentPreimage[:])
	err = db.AddPayment(fakePaymentHash, fakePayment.Invoice.Terms.Value)
	if err != nil {
		t.Fatalf("unable to put payment in DB: %v", err)
	}

	err = db.UpdatePaymentPreimage(fakePaymentPreimage)
	if err != nil {
		t.Fatalf("unable to update payment preimage: %v", err)
	}

	payments, err := db.FetchAllPayments()
	if err != nil {
		t.Fatalf("unable to fetch payments from DB: %v", err)
	}

	fakePayment.PaymentPreimage = fakePaymentPreimage

	expectedPayments := []*OutgoingPayment{fakePayment}
	if !reflect.DeepEqual(payments, expectedPayments) {
		t.Fatalf("Wrong payments after reading from DB. "+
			"Got %v, want %v",
			spew.Sdump(payments),
			spew.Sdump(expectedPayments),
		)
	}
}

func TestTotalPaymentWorkflow(t *testing.T) {
	t.Parallel()
	stubTime()
	defer unstubTime()

	db, cleanUp, err := makeTestDB()
	defer cleanUp()
	if err != nil {
		t.Fatalf("unable to make test db: %v", err)
	}

	fakePayment := makeCompleteFakePayment()
	fakePaymentHash := sha256.Sum256(fakePayment.PaymentPreimage[:])

	err = db.AddPayment(fakePaymentHash, fakePayment.Invoice.Terms.Value)
	if err != nil {
		t.Fatalf("unable to put payment in DB: %v", err)
	}

	fakePaymentRoute := &OutgoingPaymentRoute{
		Path:           fakePayment.Path,
		Fee:            fakePayment.Fee,
		TimeLockLength: fakePayment.TimeLockLength,
	}

	err = db.UpdatePaymentRoute(fakePaymentHash, fakePaymentRoute)
	if err != nil {
		t.Fatalf("unable to update payment route in DB: %v", err)
	}

	err = db.UpdatePaymentPreimage(fakePayment.PaymentPreimage)
	if err != nil {
		t.Fatalf("unable to update payment preimage: %v", err)
	}

	payments, err := db.FetchAllPayments()
	if err != nil {
		t.Fatalf("unable to fetch payments from DB: %v", err)
	}

	expectedPayments := []*OutgoingPayment{fakePayment}
	if !reflect.DeepEqual(payments, expectedPayments) {
		t.Fatalf("Wrong payments after reading from DB. "+
			"Got %v, want %v",
			spew.Sdump(payments),
			spew.Sdump(expectedPayments),
		)
	}
}

func TestPaymentStatusWorkflow(t *testing.T) {
	t.Parallel()

	db, cleanUp, err := makeTestDB()
	defer cleanUp()
	if err != nil {
		t.Fatalf("unable to make test db: %v", err)
	}

	testCases := []struct {
		paymentHash [32]byte
		status      PaymentStatus
	}{
		{
			paymentHash: makeFakePaymentHash(),
			status:      StatusGrounded,
		},
		{
			paymentHash: makeFakePaymentHash(),
			status:      StatusInFlight,
		},
		{
			paymentHash: makeFakePaymentHash(),
			status:      StatusCompleted,
		},
	}

	for _, testCase := range testCases {
		err := db.UpdatePaymentStatus(testCase.paymentHash, testCase.status)
		if err != nil {
			t.Fatalf("unable to put payment in DB: %v", err)
		}

		status, err := db.FetchPaymentStatus(testCase.paymentHash)
		if err != nil {
			t.Fatalf("unable to fetch payments from DB: %v", err)
		}

		if status != testCase.status {
			t.Fatalf("Wrong payments status after reading from DB. "+
				"Got %v, want %v",
				spew.Sdump(status),
				spew.Sdump(testCase.status),
			)
		}
	}
}
