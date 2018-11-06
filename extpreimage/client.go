package extpreimage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"

	"google.golang.org/grpc"
)

// RPC is an interface implemented by the grpc package
type RPC interface {
	Dial(host string, opt grpc.DialOption) (*grpc.ClientConn, error)
	WithInsecure() grpc.DialOption
	NewClient(*grpc.ClientConn) ExternalPreimageServiceClient
}

// grpcRpc exposes the methods from the grpc package that we need
// this allows us to stub out the grpc methods more easily
type grpcRpc struct{}

func (r *grpcRpc) Dial(host string, opt grpc.DialOption) (*grpc.ClientConn,
	error) {
	return grpc.Dial(host, opt)
}

func (r *grpcRpc) WithInsecure() grpc.DialOption {
	return grpc.WithInsecure()
}

func (r *grpcRpc) NewClient(c *grpc.ClientConn) ExternalPreimageServiceClient {
	return NewExternalPreimageServiceClient(c)
}

// DefaultRPC exposes the default gRPC implementation for consumers
func DefaultRPC() RPC {
	return &grpcRpc{}
}

// Client is the exposed interface for an extpreimage Client
type Client interface {
	connect() (ExternalPreimageServiceClient, error)
	Retrieve(*PreimageRequest) ([32]byte, error, error)
	Stop() error
}

// client is a representation of a client of the external preimage
// service that implements the Client interface
type client struct {
	host  string
	chain string
	rpc   RPC
	conn  *grpc.ClientConn
}

// connect creates a new ExternalPreimageServiceClient from an existing
// connection, or it creates a new connection if none exists.
func (c *client) connect() (ExternalPreimageServiceClient,
	error) {
	if c.conn == nil {
		conn, err := c.rpc.Dial(c.host, c.rpc.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("extpreimage: Failed to start gRPC "+
				"connection: %v", err)
		}
		fmt.Printf("extpreimage: Connected to External Preimage Service at %s\n",
			c.host)
		c.conn = conn
	} else {
		fmt.Printf("extpreimage: Re-using connection for %s\n",
			c.host)
	}

	return c.rpc.NewClient(c.conn), nil
}

// Retrieve is a wrapper around the underlying GetPreimage defined
// in rpc.proto that reduces the stream interface to a single output,
// since GetPreimage is expected to provide only one response, albeit
// after a long period of time.
// Additionally, Retrieve lazily connects to the External Preimage
// server.
func (c *client) retrieve(req *GetPreimageRequest) (*GetPreimageResponse,
	error) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := c.connect()
	if err != nil {
		return nil, err
	}

	// make the request to the server to open the stream
	stream, err := client.GetPreimage(ctx, req)
	if err != nil {
		return nil, err
	}

	for {
		// receive the next message from the stream
		res, err := stream.Recv()

		// We are expecting a single response, so if the stream closes before that
		// response, we presume that something has gone wrong.
		if err == io.EOF {
			return nil, fmt.Errorf("extpreimage: server closed stream early")
		}
		if err != nil {
			return nil, err
		}

		// We are expecting a single response, so as soon as we have it we can
		// return execution to the caller
		return res, nil
	}
}

// symbol converts the configured chain to a symbol to differentiate between
// currencies in the format that ExternalPreimage service expects.
// LND keeps track based on chainCode, but, as that value is private, we use
// its string representation to do our conversion.
func (c *client) symbol() (Symbol, error) {
	if c.chain == "bitcoin" {
		return Symbol_BTC, nil
	}

	if c.chain == "litecoin" {
		return Symbol_LTC, nil
	}

	// instantiate an empty symbol so we can pass the correct type back
	var symbol Symbol
	return symbol, fmt.Errorf("extpreimage: Invalid chain name: %v", c.chain)
}

type PreimageRequest struct {
	PaymentHash [sha256.Size]byte
	Amount      int64
	TimeLock    uint32
	BestHeight  uint32
}

// Retrieve retrieves a preimage for a given hash. It returns two errors,
// the first of which is temporary, the other is permanent. Permanent errors
// are safe to result in upstream HTLC cancellations. Temporary errors are
// not.
func (c *client) Retrieve(req *PreimageRequest) ([32]byte, error, error) {
	var preimage [32]byte

	symbol, err := c.symbol()
	if err != nil {
		// Not having the correct configuration on the chain is a temporary error
		// since we can recover from it
		return preimage, err, nil
	}

	rpcReq := &GetPreimageRequest{
		PaymentHash: req.PaymentHash[:],
		Amount:      req.Amount,
		Symbol:      symbol,
		TimeLock:    int64(req.TimeLock),
		BestHeight:  int64(req.BestHeight),
	}

	res, err := c.retrieve(rpcReq)
	if err != nil {
		// An error with retrieving the preimage itself is considered temporary
		// since we don't know if we will eventually be able to retrieve it
		return preimage, err, nil
	}

	if res.PermanentError != "" {
		// If the external service marks an error as permanent we can pass it on
		// as permanent
		return preimage, nil, fmt.Errorf("extpreimage: Encountered permanent "+
			"error from external service: %v", res.PermanentError)
	}

	if len(res.PaymentPreimage) != 32 {
		// We return this as a non-permanent error since the external service did
		// not indicate it as such
		return preimage, fmt.Errorf("extpreimage: Returned preimage was of length %v, "+
			"expected %v", len(res.PaymentPreimage), 32), nil
	}

	// Since the hash and preimage were stored separately, we need to validate that
	// this preimage actually matches this hash before returning it to the caller
	derivedHash := sha256.Sum256(res.PaymentPreimage[:])
	if !bytes.Equal(derivedHash[:], req.PaymentHash[:]) {
		// We return this as a non-permanent error since the external service did
		// not indicate it as such
		return preimage, fmt.Errorf("extpreimage: Returned preimage did not " +
			"match provided hash"), nil
	}

	copy(preimage[:], res.PaymentPreimage)
	return preimage, nil, nil
}

// Stop closes any outstanding grpc connections to allow for a graceful
// shutdown
func (c *client) Stop() error {
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

// New creates a new instance of an extpreimage Client without initiating
// a connection, so that we can lazily connect to the host
func New(RPCImpl RPC, RPCHost string, ChainName string) (Client, error) {
	if ChainName != "bitcoin" && ChainName != "litecoin" {
		return nil, fmt.Errorf("extpreimage: Invalid chain name: %v", ChainName)
	}
	c := &client{host: RPCHost, rpc: RPCImpl, chain: ChainName}
	return c, nil
}
