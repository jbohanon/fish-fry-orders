#!/bin/bash

# Create output directory
mkdir -p ui/js/proto

# Generate TypeScript types from protos
protoc \
  --plugin=protoc-gen-ts=./node_modules/.bin/protoc-gen-ts \
  --ts_out=ui/js/proto \
  --proto_path=proto \
  proto/*.proto 