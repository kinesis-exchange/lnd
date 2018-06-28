extpreimage
=====

This extpreimage package implements a client for a generic External Preimage
Service running a gRPC server.

## Generate protobuf definitions

1. Download [v.3.4.0](https://github.com/google/protobuf/releases/tag/v3.4.0) of
`protoc` for your operating system and add it to your `PATH`.
For example, if using macOS:
```bash
$ curl -LO https://github.com/google/protobuf/releases/download/v3.4.0/protoc-3.4.0-osx-x86_64.zip
$ unzip protoc-3.4.0-osx-x86_64.zip -d protoc
$ export PATH=$PWD/protoc/bin:$PATH
```

2. Install `golang/protobuf` at commit `ab9f9a6dab164b7d1246e0e688b0ab7b94d8553e`.
```bash
$ git clone https://github.com/golang/protobuf $GOPATH/src/github.com/golang/protobuf
$ cd $GOPATH/src/github.com/golang/protobuf
$ git reset --hard ab9f9a6dab164b7d1246e0e688b0ab7b94d8553e
$ make
```

3. Run [`gen_protos.sh`](./gen_protos.sh) to generate new protobuf definitions.


extpreimage_test
----------------

The `extpreimage_test` package is for testing the `extpreimage` package.

It contains auto-generated mocks that correspond with the generated files in `extpreimage`.

Generating new files is done using [`gomock`'s mockgen command](https://github.com/golang/mock#running-mockgen).

Also see the [grpc documentation on gomock](https://github.com/grpc/grpc-go/blob/master/Documentation/gomock-example.md).

### Generating Mocks

1. Install gomock

```
go get github.com/golang/mock/gomock
go install github.com/golang/mock/mockgen
```

2. Run mockgen in "Reflect mode" from the root lnd directory

```
cd $GOPATH/src/github.com/lightningnetwork/lnd/
mkdir extpreimage_test && mockgen -package=extpreimage_test github.com/lightningnetwork/lnd/extpreimage ExternalPreimageServiceClient,ExternalPreimageService_GetPreimageClient > extpreimage_test/rpc_test.go && mv extpreimage_test/rpc_test.go extpreimage/rpc_test.go && rmdir extpreimage_test
```

Note: for some reason mockgen won't generate the mocks directly into the `extpreimage` directory, which is why we generate them in a holding directory and move it.

### Using Mocks

1. Import gomock (the mocked files will already be in the `extpreimage_test` package)

```
import "github.com/golang/mock/gomock"
```

2. Use the `MockGetExternalPreimageServiceClient` as you would the `GetExternalPreimageServiceClient` client

3. Use expectations as doc'ed here: https://github.com/grpc/grpc-go/blob/master/Documentation/gomock-example.md