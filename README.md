# nvk8s-resourcemodel

This repo is meant as a staging ground for defining extensions to the
Kubernetes structured parameters model as it pertains to NVIDIA hardware.

## Building

Run `make`, it will build everything.

### Available commands
**`cmd/print-model/print-model`:**
   * This command is designed to build a k8s structured resource model from all of 
     the devices on a mock dgxa100 server and print the resource model as yaml.

## Running

Just run `make run` and it will build and run all commands in one go.
