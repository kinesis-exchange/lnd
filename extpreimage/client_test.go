package extpreimage_test

import (
	"crypto/sha256"
	"fmt"
	"io"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/lightningnetwork/lnd/extpreimage"
	grpcpool "github.com/processout/grpc-go-pool"
	"google.golang.org/grpc"
)

// rpcMsg implements the gomock.Matcher interface to allow us to EXPECT()
// specific rpc messages
type rpcMsg struct {
	msg proto.Message
}

func (r *rpcMsg) Matches(msg interface{}) bool {
	m, ok := msg.(proto.Message)
	if !ok {
		return false
	}
	return proto.Equal(m, r.msg)
}

func (r *rpcMsg) String() string {
	return fmt.Sprintf("is %s", r.msg)
}

// mockRpc is a mock implementation of extpreimage.RPC
// that allows us to test behavior without actually making RPC calls
type mockRpc struct {
	ctrl     *gomock.Controller
	conn     *grpc.ClientConn
	connOpen bool
	host     string
	expect   *rpcMsg
	stream   *MockExternalPreimageService_GetPreimageClient
}

// Dial records the destination and marks the connection as "open"
// without making an external calls
func (r *mockRpc) Dial(host string, opt grpc.DialOption) (
	*grpc.ClientConn, error) {
	conn := &grpc.ClientConn{}
	r.conn = conn
	r.connOpen = true
	r.host = host
	return conn, nil
}

func (r *mockRpc) WithInsecure() grpc.DialOption {
	return grpc.WithInsecure()
}

// NewClient returns our mock client from gomock/mockgen
// and returns the created stream to any calls to GetPreimage
func (r *mockRpc) NewClient(c *grpcpool.ClientConn) extpreimage.ExternalPreimageServiceClient {
	var expect gomock.Matcher

	if r.expect != nil {
		expect = r.expect
	} else {
		expect = gomock.Any()
	}

	client := NewMockExternalPreimageServiceClient(r.ctrl)

	// Set expectation on GetPreimage
	client.EXPECT().GetPreimage(
		gomock.Any(),
		expect,
	).Return(r.stream, nil)

	return client
}

// newMock sets up a new mock client with mock RPC
func newMock(t *testing.T, host string, chain string) (extpreimage.Client, *mockRpc) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock for the stream returned by GetPreimage
	stream := NewMockExternalPreimageService_GetPreimageClient(ctrl)

	rpc := &mockRpc{ctrl: ctrl, stream: stream}
	client, _ := extpreimage.New(rpc, host, chain)

	return client, rpc
}

func makePreimage(input string) [32]byte {
	var preimage [32]byte
	inputBytes := []byte(input)
	copy(preimage[:], inputBytes)
	return preimage
}

// TestRetrieveConnects verifies that calling Retrieve will
// connect to the remote server automatically
func TestRetrieveConnects(t *testing.T) {
	host := "mockhost:12345"
	chain := "bitcoin"
	preimage := makePreimage("fake preimage")
	hash := sha256.Sum256(preimage[:])
	msg := &extpreimage.GetPreimageResponse{
		PaymentPreimage: preimage[:],
	}
	c, rpc := newMock(t, host, chain)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(msg, nil)

	req := &extpreimage.PreimageRequest{
		PaymentHash: hash,
	}
	_, tempErr, permErr := c.Retrieve(req)

	if tempErr != nil {
		t.Fatalf("Got temporary error while retrieving: %v", tempErr)
	}

	if permErr != nil {
		t.Fatalf("Got permanent error while retrieving: %v", permErr)
	}

	if rpc.connOpen != true {
		t.Errorf("Expected conn to be open, got %v", rpc.connOpen)
	}

	if rpc.host != host {
		t.Errorf("Expected connection to %v, got %v", host, rpc.host)
	}
}

// TestRetrieveFormsValidRequest tests that the GetPreimageRequest
// is well-formed and reflects the caller's inputs.
func TestRetrieveFormsValidRequest(t *testing.T) {
	host := "mockhost:12345"
	chain := "bitcoin"
	preimage := makePreimage("fake preimage")
	hash := sha256.Sum256(preimage[:])
	amount := int64(3000)
	timeLock := uint32(10000)
	bestHeight := uint32(9000)
	msg := &extpreimage.GetPreimageResponse{
		PaymentPreimage: preimage[:],
	}

	c, rpc := newMock(t, host, chain)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(msg, nil)

	// Set expected request
	rpc.expect = &rpcMsg{
		msg: &extpreimage.GetPreimageRequest{
			PaymentHash: hash[:],
			Amount:      amount,
			Symbol:      extpreimage.Symbol_BTC,
			TimeLock:    int64(timeLock),
			BestHeight:  int64(bestHeight),
		},
	}

	req := &extpreimage.PreimageRequest{
		PaymentHash: hash,
		Amount:      amount,
		TimeLock:    timeLock,
		BestHeight:  bestHeight,
	}
	_, tempErr, permErr := c.Retrieve(req)

	if tempErr != nil {
		t.Fatalf("Got temporary error while retrieving: %v", tempErr)
	}

	if permErr != nil {
		t.Fatalf("Got permanent error while retrieving: %v", permErr)
	}
}

// TestRetrieveSuppliesSymbol tests that Retrieve supplies the
// currency symbol to the external preimage service based on
// configuration of the extpreimage client.
func TestRetrieveSuppliesSymbol(t *testing.T) {
	host := "mockhost:12345"
	chain := "litecoin"
	preimage := makePreimage("fake preimage")
	hash := sha256.Sum256(preimage[:])
	amount := int64(3000)
	timeLock := uint32(10000)
	bestHeight := uint32(9000)
	msg := &extpreimage.GetPreimageResponse{
		PaymentPreimage: preimage[:],
	}

	c, rpc := newMock(t, host, chain)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(msg, nil)

	// Set expected request
	rpc.expect = &rpcMsg{
		msg: &extpreimage.GetPreimageRequest{
			PaymentHash: hash[:],
			Amount:      amount,
			Symbol:      extpreimage.Symbol_LTC,
			TimeLock:    int64(timeLock),
			BestHeight:  int64(bestHeight),
		},
	}

	req := &extpreimage.PreimageRequest{
		PaymentHash: hash,
		Amount:      amount,
		TimeLock:    timeLock,
		BestHeight:  bestHeight,
	}
	_, tempErr, permErr := c.Retrieve(req)

	if tempErr != nil {
		t.Fatalf("Got temporary error while retrieving: %v", tempErr)
	}

	if permErr != nil {
		t.Fatalf("Got permanent error while retrieving: %v", permErr)
	}
}

// TestRetrieveReturnsFirstMessage verifies that as soon as a single
// response is received from the ExternalPreimageService, it will
// be returned to the caller
func TestRetrieveReturnsFirstMessage(t *testing.T) {
	host := "mockhost:12345"
	chain := "bitcoin"
	preimage := makePreimage("fake preimage")
	hash := sha256.Sum256(preimage[:])
	msg := &extpreimage.GetPreimageResponse{
		PaymentPreimage: preimage[:],
	}
	c, rpc := newMock(t, host, chain)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(msg, nil)

	req := &extpreimage.PreimageRequest{
		PaymentHash: hash,
	}
	res, tempErr, permErr := c.Retrieve(req)

	if tempErr != nil {
		t.Fatalf("Got temporary error while retrieving: %v", tempErr)
	}

	if permErr != nil {
		t.Fatalf("Got permanent error while retrieving: %v", permErr)
	}

	if res != preimage {
		t.Fatalf("Expected preimage of %v, got %v", preimage, res)
	}
}

// TestRetrievesRejectsInvalidPreimages tests that Retrieve will return an
// error if the external preimage service returns a preimage that does not
// match the provided hash.
func TestRetrievesRejectsInvalidPreimages(t *testing.T) {
	host := "mockhost:12345"
	chain := "bitcoin"
	preimage := makePreimage("fake preimage")
	otherPreimage := makePreimage("another preimage")
	hash := sha256.Sum256(otherPreimage[:])
	expectedErr := "extpreimage: Returned preimage did not match provided hash"
	msg := &extpreimage.GetPreimageResponse{
		PaymentPreimage: preimage[:],
	}
	c, rpc := newMock(t, host, chain)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(msg, nil)

	req := &extpreimage.PreimageRequest{
		PaymentHash: hash,
	}
	_, tempErr, permErr := c.Retrieve(req)

	if permErr != nil {
		t.Fatalf("Got permanent error while retrieving: %v", permErr)
	}

	if (tempErr == nil) || (tempErr.Error() != expectedErr) {
		t.Fatalf("Expected tempErr of %v, got %v", expectedErr, tempErr)
	}
}

// TestRetrieveErrorsOnStreamError tests that Retrieve will return
// an error to the caller if the stream to the external preimage
// server encounters an error.
func TestRetrieveTempErrorsOnStreamError(t *testing.T) {
	host := "mockhost:12345"
	chain := "bitcoin"
	fakeErr := fmt.Errorf("fake error")

	c, rpc := newMock(t, host, chain)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(nil, fakeErr)

	req := &extpreimage.PreimageRequest{}
	_, tempErr, permErr := c.Retrieve(req)

	if permErr != nil {
		t.Fatalf("Got permanent error while retrieving: %v", permErr)
	}

	if tempErr != fakeErr {
		t.Fatalf("Expected tempErr of %v, got %v", fakeErr, tempErr)
	}
}

// TestRetrieveErrorsOnEarlyClose tests that Retrieve will return an error
// to the caller if the stream to the external preimage service closes
// before returning the requested preimage.
func TestRetrieveTempErrorsOnEarlyClose(t *testing.T) {
	host := "mockhost:12345"
	chain := "bitcoin"
	expectedErr := "extpreimage: server closed stream early"

	c, rpc := newMock(t, host, chain)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(nil, io.EOF)

	req := &extpreimage.PreimageRequest{}
	_, tempErr, permErr := c.Retrieve(req)

	if permErr != nil {
		t.Fatalf("Got permanent error while retrieving: %v", permErr)
	}

	if (tempErr == nil) || (tempErr.Error() != expectedErr) {
		t.Fatalf("Expected tempErr of %v, got %v", expectedErr, tempErr.Error())
	}
}

// TestRetrievePermanentErrorsOnRoutingFailure tests that Retrieve will return
// a permanent error if it receives a permanent failure from the external
// preimage service.
func TestRetrievePermanentErrorsOnPermanentFailure(t *testing.T) {
	host := "mockhost:12345"
	chain := "bitcoin"
	err := "fake error"
	preimage := makePreimage("fake preimage")
	hash := sha256.Sum256(preimage[:])
	expectedErr := "extpreimage: Encountered permanent error from external "+
		"service: " + err
	msg := &extpreimage.GetPreimageResponse{
		PermanentError: err,
	}
	c, rpc := newMock(t, host, chain)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(msg, nil)

	req := &extpreimage.PreimageRequest{
		PaymentHash: hash,
	}
	_, tempErr, permErr := c.Retrieve(req)

	if tempErr != nil {
		t.Fatalf("Got temporary error while retrieving: %v", tempErr)
	}

	if (permErr == nil) || (permErr.Error() != expectedErr) {
		t.Fatalf("Expected permErr of %v, got %v", expectedErr, permErr)
	}
}
