# nvk8s-resourcemodel

This repo is meant as a staging ground for defining extensions to the
Kubernetes structured parameters model as it pertains to NVIDIA hardware.
All of the running examples use a Mock DGXA100 server to feed its input, so the
results it generates are comparable to what we would see in real hardware.

## Proposed Extensions

Below is a summary of the currently proposed extensions. With these extensions
in place we are able to enable fully dynamic MIG with the possibility for the
scheduler to do intelligent allocation to avoid fragmentation.

```diff
 // NamedResourcesResources is used in ResourceModel.
 // +k8s:deepcopy-gen=true
 type NamedResourcesResources struct {
 	// The list of all individual resources instances currently available.
 	//
 	// +listType=atomic
 	Instances []NamedResourcesInstance `json:"instances" protobuf:"bytes,1,name=instances"`
 
+	// The list of all shared resources limits that are referenced by one or
+	// more instances.
+	//
+	// +listType=atomic
+	// +optional
+	SharedLimits []NamedResourcesGroup `json:"sharedLimits,omitempty" protobuf:"bytes,2,opt,name=sharedLimits"`
 }
 
 // NamedResourcesInstance represents one individual hardware instance that can be selected based
 // on its attributes.
 // +k8s:deepcopy-gen=true
 type NamedResourcesInstance struct {
 	// Name is unique identifier among all resource instances managed by
 	// the driver on the node. It must be a DNS subdomain.
 	Name string `json:"name" protobuf:"bytes,1,name=name"`
 
 	// Attributes defines the attributes of this resource instance.
 	// The name of each attribute must be unique.
 	//
 	// +listType=atomic
 	// +optional
 	Attributes []NamedResourcesAttribute `json:"attributes,omitempty" protobuf:"bytes,2,opt,name=attributes"`
 
+	// Resources defines the set of resources this instance consumes when
+	// allocated.
+	//
+	// +listType=atomic
+	// +optional
+	Resources []NamedResourcesGroup `json:"resources,omitempty" protobuf:"bytes,3,opt,name=resources"`
 }
 
+// NamedResourcesQuantity represents a named quantity of resources.
+// +k8s:deepcopy-gen=true
+type NamedResourcesQuantity struct {
+	// Name is the name of the resource represented by this quantity.
+	// It must be a DNS subdomain.
+	Name string `json:"name" protobuf:"bytes,1,name=name"`
+
+	// Value is the actual quantity of resources.
+	Value *resource.Quantity `json:"value" protobuf:"bytes,2,name=value"`
+}
+
+// NamedResourcesIntSet represents a named list of discrete integers.
+// +k8s:deepcopy-gen=true
+type NamedResourcesIntSet struct {
+	// Name is the name of the resource represented by this quantity.
+	// It must be a DNS subdomain.
+	Name string `json:"name" protobuf:"bytes,1,name=name"`
+
+	// Items is the actual set of ints.
+	//
+	// +listType=set
+	Items []int `json:"items" protobuf:"bytes,2,name=items"`
+}
+
+// NamedResourcesStringSet represents a named list of discrete strings.
+// +k8s:deepcopy-gen=true
+type NamedResourcesStringSet struct {
+	// Name is the name of the resource represented by this quantity.
+	// It must be a DNS subdomain.
+	Name string `json:"name" protobuf:"bytes,1,name=name"`
+
+	// Items is the actual set of strings.
+	//
+	// +listType=set
+	Items []string `json:"items" protobuf:"bytes,2,name=items"`
+}
+
+// NamedResourcesNamedResourceGroup represents a named group of resources (quantites and sets).
+// +k8s:deepcopy-gen=true
+type NamedResourcesGroup struct {
+	// Name is unique identifier among all resource groups managed by
+	// the driver on the node. It must be a DNS subdomain.
+	Name string `json:"name" protobuf:"bytes,1,name=name"`
+
+	// Quantities represents the list of all named resource quantities in the
+	// resource group.
+	//
+	// +listType=atomic
+	// +optional
+	Quantities []NamedResourcesQuantity `json:"quantities,omitempty" protobuf:"bytes,2,opt,name=quantities"`
+
+	// StringSets represents the list of all named resource sets that contains
+	// strings in the resource group.
+	//
+	// +listType=atomic
+	// +optional
+	StringSets []NamedResourcesStringSet `json:"stringSets,omitempty" protobuf:"bytes,3,opt,name=stringSets"`
+
+	// IntSets represents the list of all named resource sets that contains
+	// ints in the resource group.
+	//
+	// +listType=atomic
+	// +optional
+	IntSets []NamedResourcesIntSet `json:"intSets,omitempty" protobuf:"bytes,4,opt,name=intSets"`
+}
```

The basic idea is the following:

* Introduce a top-level `SharedLimits` field in the `NamedResources` struct.
  This field is a list of a new type called `NamedResourcesGroups`, each of
  wich defines a named collection of resources and their limits. In the case of
  NVIDIA GPUs, there will be one `NamedResourcesGroup` per GPU on the machine,
  whose name is `gpu-%d-shared-resources` and has limits set for all of the
  "sub-resources" that can be consumed by different MIG devices of that GPU.

* Introduce a `Resources` field in the `NamedResourcesInstance` struct.
  This field is also a list of `NamedResourcesGroups`, but instead of defining
  a _limit_ of available resources as before, it defines the actual _set_ of
  resources that would be consumed by this instance if it were to be allocated.
  In the case of NVIDI GPUs, we declare one instance for every possible MIG
  device or full GPU that could be allocated from the machine. In turn, each of
  these instances declare resources for the total amount of memory they consume,
  the discrete set of memory slices they consume (to help avoid fragmentation
  later), as well as the number of Jpeg, Ofa, Decoder engines, etc. that will be
  consumed if they get allocated.

With these simple additions in place, we have everyting we need to support
fully dynamic partitoining of GPUs with MIG. In essence, the scheduler is now
able to track the consumption of any shared (i.e. overlapping) resources and
ensure that their limits are not exceeded when making a scheduling decision.
In the case of NVIDI GPUs, this means that overlapping MIG devices (as well as
the full GPUs they are part of) can be considered by the scheduler
independently, without it needing to understand the exact device hierarchy of
the hardware itself.

## Examples / Proof of Concept

There are two different commands available:

**`print-model`**:
    This command will print out two resource models for GPU 0 on the Mock
    DGXA100 server. The first is defined using the `NamedResources` model in
    Kubernetes v1.30. The second is with a set of proposed extensions to
    support dynamic partitioning of MIG devices.

**`print-possible-allocations`**:
    This command will print out all possible allocations of the resources
    declared for GPU 0 on the Mock DGXA100 server using the proposed extensions
    to the `NamedResources` model. This is meant to demonstrate how the
    scheduler can use these extensions to dynamically allocate non-overlapping
    devices partitioned from the same piece of hardware.

To run these commands, invoke `make <cmd-name>`.

The output of running each command can be seen below:

```console
$ make print-model

######## NamedResourceModel v1.30 ########
namedResources:
  instances:
  - attributes:
    - int: 0
      name: minor
    - int: 0
      name: index
    - name: uuid
      string: GPU-dda8efb4-418d-4a97-9612-630e57e0eee2
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
      string: GPU-dda8efb4-418d-4a97-9612-630e57e0eee2
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
    - name: gpu-0-shared-resources
      quantities:
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
    - name: memory
      value: 40Gi
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
```

```console
$ make print-possible-allocations

[gpu-0]
[gpu-0-mig-1g.5gb-0]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-3g.20gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-3g.20gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-3g.20gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-3g.20gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-3g.20gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-3g.20gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-6]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-3g.20gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-3g.20gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-0]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-3g.20gb-0]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-2g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-3g.20gb-0]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-3g.20gb-4]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-3g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-3g.20gb-4]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-4g.20gb-0]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-3g.20gb-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-4g.20gb-0]
[gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-4g.20gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-7g.40gb-0]
[gpu-0-mig-1g.5gb-me-0]
[gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-1]
[gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-1 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-2]
[gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-3]
[gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-3 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-4]
[gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-5]
[gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-5 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.5gb-me-6]
[gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.5gb-me-6 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.10gb-0]
[gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.10gb-0 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.10gb-2]
[gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.10gb-2 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.10gb-4]
[gpu-0-mig-1g.10gb-4 gpu-0-mig-1g.10gb-6]
[gpu-0-mig-1g.10gb-6]
```
