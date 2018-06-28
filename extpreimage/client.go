package extpreimage

import (
	"context"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	grpcpool "github.com/processout/grpc-go-pool"
)

// RPC is an interface implemented by the grpc package
type RPC interface {
	Dial(host string, opt grpc.DialOption) (*grpc.ClientConn, error)
	WithInsecure() grpc.DialOption
	NewClient(*grpcpool.ClientConn) (ExternalPreimageServiceClient)
}

// grpcRpc exposes the methods from the grpc package that we need
// this allows us to stub out the grpc methods more easily
type grpcRpc struct {}

func (r *grpcRpc) Dial(host string, opt grpc.DialOption) (*grpc.ClientConn,
	error) {
  return grpc.Dial(host, opt)
}

func (r *grpcRpc) WithInsecure() grpc.DialOption {
  return grpc.WithInsecure()
}

func (r *grpcRpc) NewClient(c *grpcpool.ClientConn) (
	ExternalPreimageServiceClient) {
	return NewExternalPreimageServiceClient(c.ClientConn)
}

// DefaultRPC exposes the default gRPC implementation for consumers
func DefaultRPC() RPC {
	return &grpcRpc{}
}

// Client is the exposed interface for an extpreimage Client
type Client interface {
	connect(context.Context) (ExternalPreimageServiceClient, error)
	Retrieve(*GetPreimageRequest) (*GetPreimageResponse, error)
	Stop()
}

// client is a representation of a client of the external preimage
// service that implements the Client interface
type client struct {
	host string
	conn *grpc.ClientConn
	client ExternalPreimageServiceClient
	rpc RPC
	pool *grpcpool.Pool
}

// connect creates a new ExternalPreimageServiceClient from the connection pool
func (c *client) connect(ctx context.Context) (ExternalPreimageServiceClient,
	error) {
	conn, err := c.pool.Get(ctx); if err != nil {
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
func (c *client) Retrieve(req *GetPreimageRequest) (*GetPreimageResponse,
	error) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client, err := c.connect(ctx); if err != nil {
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

// Stop closes any outstanding grpc connections to allow for a graceful shutdown
func (c *client) Stop() {
	c.pool.Close()
}

// newPool creates a new grpc pool of connections for the client to use
func newPool(c *client) (*grpcpool.Pool, error) {
	var factory grpcpool.Factory

	// factory creates new Connections to be used by the pool
	factory = func() (*grpc.ClientConn, error) {
		conn, err := c.rpc.Dial(c.host, c.rpc.WithInsecure()); if err != nil {
			return nil, fmt.Errorf("extpreimage: Failed to start gRPC connection: "+
				"%v",err)
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
func New(RPCImpl RPC, RPCHost string) (Client, error) {
	c := &client{host: RPCHost, rpc: RPCImpl}
	var err error
	c.pool, err = newPool(c); if err != nil {
		return nil, err
	}
	return c, nil
}
