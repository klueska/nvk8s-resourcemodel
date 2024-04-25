package main

import (
	"fmt"

	"github.com/NVIDIA/go-nvml/pkg/nvml/mock/dgxa100"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"

	nvdevicelib "github.com/klueska/nvk8s-resourcemodel/pkg/nvdevice"
	currentresourceapi "github.com/klueska/nvk8s-resourcemodel/pkg/resource/current"
	newresourceapi "github.com/klueska/nvk8s-resourcemodel/pkg/resource/new"
)

// Main queries the list of allocatable devices and prints them as a kubernetes
// structure resource model.
func main() {
	// Instantiate an instance of a mock dgxa100 server and build a nvDeviceLib
	// from it. The nvDeviceLib is then used to populate the list of allocatable
	// devices from this mock server using standard NVML calls.
	l := nvdevicelib.New(dgxa100.New())

	// Get the full list of allocatable devices from the server.
	allocatable, err := l.GetAllocatableDevices(0)
	if err != nil {
		klog.Fatalf("Error getAllocatableDevices: %v", err)
	}

	// Print the current structured resource model.
	fmt.Printf("######## NamedResourceModel v1.30 ########\n")
	if err := printCurrentResourceModel(allocatable); err != nil {
		klog.Fatalf("Error printCurrentResourceModel: %v", err)
	}

	fmt.Printf("\n")

	// Print the new structured resource model.
	fmt.Printf("######## Proposed NamedResourceModel v1.31 ########\n")
	if err := printNewResourceModel(allocatable); err != nil {
		klog.Fatalf("Error printNewResourceModel: %v", err)
	}
}

// printCurrentResourceModel prints the current structured resource model as yaml.
func printCurrentResourceModel(allocatable nvdevicelib.AllocatableDevices) error {
	// Build a structured resource model from the list of allocatable devices.
	instances := currentresourceapi.AllocatableDevices(allocatable).ToNamedResourceInstances()
	model := currentresourceapi.ResourceModel{
		NamedResources: &currentresourceapi.NamedResourcesResources{Instances: instances},
	}

	// Print the structured resource model as yaml.
	modelYaml, err := yaml.Marshal(model)
	if err != nil {
		klog.Fatalf("Error marshaling resource model to yaml: %v", err)
	}
	fmt.Printf("%v", string(modelYaml))

	return nil
}

// printCurrentResourceModel prints the new structured resource model as yaml.
func printNewResourceModel(allocatable nvdevicelib.AllocatableDevices) error {
	// Build a structured resource model from the list of allocatable devices.
	instances := newresourceapi.AllocatableDevices(allocatable).ToNamedResourceInstances()
	sharedLimits := newresourceapi.AllocatableDevices(allocatable).ToSharedLimits()
	model := newresourceapi.ResourceModel{
		NamedResources: &newresourceapi.NamedResourcesResources{
			Instances:    instances,
			SharedLimits: sharedLimits,
		},
	}

	// Print the structured resource model as yaml.
	modelYaml, err := yaml.Marshal(model)
	if err != nil {
		klog.Fatalf("Error marshaling resource model to yaml: %v", err)
	}
	fmt.Printf("%v", string(modelYaml))

	return nil
}
