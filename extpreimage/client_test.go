package extpreimage_test

import (
	"testing"
	"fmt"
	"io"

	"github.com/golang/mock/gomock"
	"github.com/lightningnetwork/lnd/extpreimage"
	"google.golang.org/grpc"
	grpcpool "github.com/processout/grpc-go-pool"
)

// mockRpc is a mock implementation of extpreimage.RPC
// that allows us to test behavior without actually making RPC calls
type mockRpc struct {
	ctrl *gomock.Controller
	conn *grpc.ClientConn
	connOpen bool
	host string
	stream *MockExternalPreimageService_GetPreimageClient
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
func (r *mockRpc) NewClient(c *grpcpool.ClientConn) (
	extpreimage.ExternalPreimageServiceClient) {
	client := NewMockExternalPreimageServiceClient(r.ctrl)

	// Set expectation on GetPreimage
	client.EXPECT().GetPreimage(
		gomock.Any(),
		gomock.Any(),
	).Return(r.stream, nil)

	return client
}

// newMock sets up a new mock client with mock RPC
func newMock(t *testing.T, host string) (extpreimage.Client, *mockRpc) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// Create mock for the stream returned by GetPreimage
	stream := NewMockExternalPreimageService_GetPreimageClient(ctrl)

	rpc := &mockRpc{ctrl: ctrl, stream: stream}
	client, _ := extpreimage.New(rpc, host)

	return client, rpc
}

// TestRetrieveConnects verifies that calling Retrieve will
// connect to the remote server automatically
func TestRetrieveConnects(t *testing.T) {
	host := "mockhost:12345"
	c, rpc := newMock(t, host)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(nil, nil)

	req := &extpreimage.GetPreimageRequest{}
	_, err := c.Retrieve(req)

	if err != nil {
		t.Fatalf("Got error while retrieving: %v", err)
	}

	if rpc.connOpen != true {
		t.Errorf("Expected conn to be open, got %v", rpc.connOpen)
	}

	if rpc.host != host {
		t.Errorf("Expected connection to %v, got %v", host, rpc.host)
	}
}

// TestRetrieveReturnsFirstMessage verifies that as soon as a single
// response is received from the ExternalPreimageService, it will
// be returned to the caller
func TestRetrieveReturnsFirstMessage(t *testing.T) {
	host := "mockhost:12345"
	msg := &extpreimage.GetPreimageResponse{
		PaymentPreimage: []byte("fake preimage"),
	}

	c, rpc := newMock(t, host)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(msg, nil)

	req := &extpreimage.GetPreimageRequest{}
	res, err := c.Retrieve(req)

	if err != nil {
		t.Fatalf("Got error while retrieving: %v", err)
	}

	if res != msg {
		t.Fatalf("Expected res of %v, got %v", msg, res)
	}
}

func TestRetrieveErrorsOnStreamError(t *testing.T) {
	host := "mockhost:12345"
	fakeErr := fmt.Errorf("fake error")

	c, rpc := newMock(t, host)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(nil, fakeErr)

	req := &extpreimage.GetPreimageRequest{}
	_, err := c.Retrieve(req)

	if err != fakeErr {
		t.Fatalf("Expected err of %v, got %v", fakeErr, err)
	}
}

func TestRetrieveErrorsOnEarlyClose(t *testing.T) {
	host := "mockhost:12345"
	expectedErr := "ExternalPreimageServiceClient: server closed stream early"

	c, rpc := newMock(t, host)

	// Set expectation on receiving.
	rpc.stream.EXPECT().Recv().Return(nil, io.EOF)

	req := &extpreimage.GetPreimageRequest{}
	_, err := c.Retrieve(req)

	if err.Error() != expectedErr {
		t.Fatalf("Expected err of %v, got %v", expectedErr, err.Error())
	}
}