# nvk8s-resourcemodel

This repo is meant as a staging ground for defining extensions to the
Kubernetes structured parameters model as it pertains to NVIDIA hardware.

## Running

Run `make run` to build and run all commands in one go.

```console
$ make run

######## NamedResourceModel v1.30 ########
namedResources:
  instances:
  - attributes:
    - int: 0
      name: minor
    - int: 0
      name: index
    - name: uuid
      string: GPU-650f49e1-eb8f-4d88-8756-776ef8f06151
    - name: memory
      quantity: 40Gi
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    - bool: true
      name: mig-capable
    - bool: false
      name: mig-enabled
    name: gpu-0

######## Proposed NamedResourceModel v1.31 ########
namedResources:
  instances:
  - attributes:
    - int: 0
      name: minor
    - int: 0
      name: index
    - name: uuid
      string: GPU-650f49e1-eb8f-4d88-8756-776ef8f06151
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    - bool: true
      name: mig-capable
    - bool: false
      name: mig-enabled
    name: gpu-0
    resources:
    - intSets:
      - items:
        - 0
        - 1
        - 2
        - 3
        - 4
        - 5
        - 6
        - 7
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "98"
      - name: copy-engines
        value: "7"
      - name: decoders
        value: "5"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "1"
      - name: ofa-engines
        value: "1"
      - name: memory
        value: 40Gi
  - attributes:
    - name: mig-profile
      string: 1g.5gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-0
    resources:
    - intSets:
      - items:
        - 0
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "0"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-1
    resources:
    - intSets:
      - items:
        - 1
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "0"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-2
    resources:
    - intSets:
      - items:
        - 2
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "0"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-3
    resources:
    - intSets:
      - items:
        - 3
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "0"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-4
    resources:
    - intSets:
      - items:
        - 4
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "0"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-5
    resources:
    - intSets:
      - items:
        - 5
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "0"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-6
    resources:
    - intSets:
      - items:
        - 6
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "0"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 2g.10gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-2g.10gb-0
    resources:
    - intSets:
      - items:
        - 0
        - 1
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "28"
      - name: copy-engines
        value: "2"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 9856Mi
  - attributes:
    - name: mig-profile
      string: 2g.10gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-2g.10gb-2
    resources:
    - intSets:
      - items:
        - 2
        - 3
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "28"
      - name: copy-engines
        value: "2"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 9856Mi
  - attributes:
    - name: mig-profile
      string: 2g.10gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-2g.10gb-4
    resources:
    - intSets:
      - items:
        - 4
        - 5
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "28"
      - name: copy-engines
        value: "2"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 9856Mi
  - attributes:
    - name: mig-profile
      string: 3g.20gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-3g.20gb-0
    resources:
    - intSets:
      - items:
        - 0
        - 1
        - 2
        - 3
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "42"
      - name: copy-engines
        value: "3"
      - name: decoders
        value: "2"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 19968Mi
  - attributes:
    - name: mig-profile
      string: 3g.20gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-3g.20gb-4
    resources:
    - intSets:
      - items:
        - 4
        - 5
        - 6
        - 7
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "42"
      - name: copy-engines
        value: "3"
      - name: decoders
        value: "2"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 19968Mi
  - attributes:
    - name: mig-profile
      string: 4g.20gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-4g.20gb-0
    resources:
    - intSets:
      - items:
        - 0
        - 1
        - 2
        - 3
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "56"
      - name: copy-engines
        value: "4"
      - name: decoders
        value: "2"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 19968Mi
  - attributes:
    - name: mig-profile
      string: 7g.40gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-7g.40gb-0
    resources:
    - intSets:
      - items:
        - 0
        - 1
        - 2
        - 3
        - 4
        - 5
        - 6
        - 7
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "98"
      - name: copy-engines
        value: "7"
      - name: decoders
        value: "5"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "1"
      - name: ofa-engines
        value: "1"
      - name: memory
        value: 40192Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb+me
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-me-0
    resources:
    - intSets:
      - items:
        - 0
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "1"
      - name: ofa-engines
        value: "1"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb+me
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-me-1
    resources:
    - intSets:
      - items:
        - 1
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "1"
      - name: ofa-engines
        value: "1"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb+me
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-me-2
    resources:
    - intSets:
      - items:
        - 2
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "1"
      - name: ofa-engines
        value: "1"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb+me
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-me-3
    resources:
    - intSets:
      - items:
        - 3
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "1"
      - name: ofa-engines
        value: "1"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb+me
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-me-4
    resources:
    - intSets:
      - items:
        - 4
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "1"
      - name: ofa-engines
        value: "1"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb+me
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-me-5
    resources:
    - intSets:
      - items:
        - 5
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "1"
      - name: ofa-engines
        value: "1"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.5gb+me
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.5gb-me-6
    resources:
    - intSets:
      - items:
        - 6
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "1"
      - name: ofa-engines
        value: "1"
      - name: memory
        value: 4864Mi
  - attributes:
    - name: mig-profile
      string: 1g.10gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.10gb-0
    resources:
    - intSets:
      - items:
        - 0
        - 1
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 9856Mi
  - attributes:
    - name: mig-profile
      string: 1g.10gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.10gb-2
    resources:
    - intSets:
      - items:
        - 2
        - 3
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 9856Mi
  - attributes:
    - name: mig-profile
      string: 1g.10gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.10gb-4
    resources:
    - intSets:
      - items:
        - 4
        - 5
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 9856Mi
  - attributes:
    - name: mig-profile
      string: 1g.10gb
    - name: product-name
      string: Mock NVIDIA A100-SXM4-40GB
    - name: brand
      string: Nvidia
    - name: architecture
      string: Ampere
    - name: cuda-compute-capability
      version: 8.0.0
    - name: driver-version
      version: 550.54.15
    - name: cuda-driver-version
      version: 12.4.0
    name: gpu-0-mig-1g.10gb-6
    resources:
    - intSets:
      - items:
        - 6
        - 7
        name: memory-slices
      name: gpu-0-shared-resources
      quantities:
      - name: multiprocessors
        value: "14"
      - name: copy-engines
        value: "1"
      - name: decoders
        value: "1"
      - name: encoders
        value: "0"
      - name: jpeg-engines
        value: "0"
      - name: ofa-engines
        value: "0"
      - name: memory
        value: 9856Mi
  sharedLimits:
  - intSets:
    - items:
      - 0
      - 1
      - 2
      - 3
      - 4
      - 5
      - 6
      - 7
      name: memory-slices
    name: gpu-0-shared-resources
    quantities:
    - name: multiprocessors
      value: "98"
    - name: copy-engines
      value: "7"
    - name: decoders
      value: "5"
    - name: encoders
      value: "0"
    - name: jpeg-engines
      value: "1"
    - name: ofa-engines
      value: "1"
    - name: memory
      value: 40Gi
```
