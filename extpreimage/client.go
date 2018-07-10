package extpreimage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"time"

	grpcpool "github.com/processout/grpc-go-pool"
	"google.golang.org/grpc"
)

// RPC is an interface implemented by the grpc package
type RPC interface {
	Dial(host string, opt grpc.DialOption) (*grpc.ClientConn, error)
	WithInsecure() grpc.DialOption
	NewClient(*grpcpool.ClientConn) ExternalPreimageServiceClient
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

func (r *grpcRpc) NewClient(c *grpcpool.ClientConn) ExternalPreimageServiceClient {
	return NewExternalPreimageServiceClient(c.ClientConn)
}

// DefaultRPC exposes the default gRPC implementation for consumers
func DefaultRPC() RPC {
	return &grpcRpc{}
}

// Client is the exposed interface for an extpreimage Client
type Client interface {
	connect(context.Context) (ExternalPreimageServiceClient, error)
	Retrieve(*PreimageRequest) ([32]byte, error)
	Stop()
}

// client is a representation of a client of the external preimage
// service that implements the Client interface
type client struct {
	host   string
	chain  string
	conn   *grpc.ClientConn
	client ExternalPreimageServiceClient
	rpc    RPC
	pool   *grpcpool.Pool
}

// connect creates a new ExternalPreimageServiceClient from the connection pool
func (c *client) connect(ctx context.Context) (ExternalPreimageServiceClient,
	error) {
	conn, err := c.pool.Get(ctx)
	if err != nil {
		return nil, err
	}
	return c.rpc.NewClient(conn), nil
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

	client, err := c.connect(ctx)
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

func (c *client) Retrieve(req *PreimageRequest) ([32]byte, error) {
	var preimage [32]byte

	symbol, err := c.symbol()
	if err != nil {
		return preimage, err
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
		return preimage, err
	}

	if len(res.PaymentPreimage) != 32 {
		return preimage, fmt.Errorf("extpreimage: Returned preimage was of length %v, "+
			"expected %v", len(res.PaymentPreimage), 32)
	}

	// Since the hash and preimage were stored separately, we need to validate that
	// this preimage actually matches this hash before returning it to the caller
	derivedHash := sha256.Sum256(res.PaymentPreimage[:])
	if !bytes.Equal(derivedHash[:], req.PaymentHash[:]) {
		return preimage, fmt.Errorf("extpreimage: Returned preimage did not " +
			"match provided hash")
	}

	copy(preimage[:], res.PaymentPreimage)
	return preimage, nil
}

// Stop closes any outstanding grpc connections to allow for a graceful
// shutdown
func (c *client) Stop() {
	c.pool.Close()
}

// newPool creates a new grpc pool of connections for the client to use
func newPool(c *client) (*grpcpool.Pool, error) {
	var factory grpcpool.Factory

	// factory creates new Connections to be used by the pool
	factory = func() (*grpc.ClientConn, error) {
		conn, err := c.rpc.Dial(c.host, c.rpc.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("extpreimage: Failed to start gRPC connection: "+
				"%v", err)
		}
		fmt.Printf("extpreimage: Connected to External Preimage Service at %s\n",
			c.host)
		return conn, err
	}

	// limit the maximum number of connections to the extpreimage server to 5
	return grpcpool.New(factory, 5, 5, time.Second)
}

// New creates a new instance of an extpreimage Client without initiating
// a connection, so that we can lazily connect to the host
func New(RPCImpl RPC, RPCHost string, ChainName string) (Client, error) {
	if ChainName != "bitcoin" && ChainName != "litecoin" {
		return nil, fmt.Errorf("extpreimage: Invalid chain name: %v", ChainName)
	}
	c := &client{host: RPCHost, rpc: RPCImpl, chain: ChainName}
	var err error
	c.pool, err = newPool(c)
	if err != nil {
		return nil, err
	}
	return c, nil
}
