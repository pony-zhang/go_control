package hal

import (
	"context"
	"fmt"
	"sync"
	"time"

	"control/pkg/types"
	"control/internal/logging"
)

// ResourceManager manages hardware resources and their allocation
type ResourceManager struct {
	resources     map[string]map[string]*Resource // resourceType -> resourceID -> Resource
	resourcesLock sync.RWMutex
	config       types.SystemConfig
	logger       *logging.Logger
	ctx          context.Context
}

// Resource represents a hardware resource with allocation state
type Resource struct {
	ID           string
	Type         string
	Allocated    bool
	AllocatedTo  string
	AllocatedAt  time.Time
	LastUsed     time.Time
	Metadata     map[string]interface{}
}

// NewResourceManager creates a new resource manager
func NewResourceManager(config types.SystemConfig) *ResourceManager {
	return &ResourceManager{
		resources: make(map[string]map[string]*Resource),
		config:    config,
		logger:    logging.GetLogger("hal_resource_manager"),
	}
}

// Start initializes the resource manager
func (rm *ResourceManager) Start(ctx context.Context) error {
	rm.ctx = ctx
	rm.logger.Info("Starting HAL Resource Manager")

	// Initialize hardware resources based on configuration
	if err := rm.initializeHardwareResources(); err != nil {
		return fmt.Errorf("failed to initialize hardware resources: %w", err)
	}

	rm.logger.Info("HAL Resource Manager started successfully")
	return nil
}

// Stop gracefully shuts down the resource manager
func (rm *ResourceManager) Stop() error {
	rm.logger.Info("Stopping HAL Resource Manager")

	rm.resourcesLock.Lock()
	defer rm.resourcesLock.Unlock()

	// Release all allocated resources
	for resourceType, resources := range rm.resources {
		for resourceID, resource := range resources {
			if resource.Allocated {
				rm.logger.Warn("Force releasing allocated resource during shutdown",
					"resource_type", resourceType, "resource_id", resourceID, "allocated_to", resource.AllocatedTo)
				resource.Allocated = false
				resource.AllocatedTo = ""
			}
		}
	}

	rm.logger.Info("HAL Resource Manager stopped successfully")
	return nil
}

// initializeHardwareResources initializes hardware resources from configuration
func (rm *ResourceManager) initializeHardwareResources() error {
	rm.logger.Info("Initializing hardware resources")

	// Initialize axis resources
	for axisID, axisConfig := range rm.config.Axes {
		rm.logger.Info("Registering axis resource", "axis_id", axisID)
		rm.registerResource("axis", string(axisID), map[string]interface{}{
			"device_id":    axisConfig.DeviceID,
			"min_position": axisConfig.MinPosition,
			"max_position": axisConfig.MaxPosition,
			"max_velocity": axisConfig.MaxVelocity,
			"max_acceleration": axisConfig.MaxAcceleration,
		})
	}

	// Initialize device resources
	for deviceID, deviceConfig := range rm.config.Devices {
		rm.logger.Info("Registering device resource", "device_id", deviceID)
		rm.registerResource("device", string(deviceID), map[string]interface{}{
			"type":     deviceConfig.Type,
			"protocol": deviceConfig.Protocol,
			"endpoint": deviceConfig.Endpoint,
		})
	}

	// Initialize safety resources
	rm.logger.Info("Registering safety resources")
	rm.registerResource("safety", "emergency_stop", map[string]interface{}{
		"enabled": rm.config.Safety.EnableEmergency,
	})
	rm.registerResource("safety", "safety_door", map[string]interface{}{
		"enabled": true, // Safety door always enabled as a basic safety feature
	})
	rm.registerResource("safety", "limit_switches", map[string]interface{}{
		"enabled": rm.config.Safety.EnableLimits,
	})

	rm.logger.Info("Hardware resources initialized successfully")
	return nil
}

// registerResource registers a hardware resource
func (rm *ResourceManager) registerResource(resourceType string, resourceID string, metadata map[string]interface{}) {
	rm.resourcesLock.Lock()
	defer rm.resourcesLock.Unlock()

	if _, exists := rm.resources[resourceType]; !exists {
		rm.resources[resourceType] = make(map[string]*Resource)
	}

	rm.resources[resourceType][resourceID] = &Resource{
		ID:          resourceID,
		Type:        resourceType,
		Allocated:   false,
		AllocatedTo: "",
		LastUsed:    time.Now(),
		Metadata:    metadata,
	}
}

// Allocate allocates a hardware resource
func (rm *ResourceManager) Allocate(resourceType string, resourceID string) error {
	rm.resourcesLock.Lock()
	defer rm.resourcesLock.Unlock()

	resourceTypeMap, exists := rm.resources[resourceType]
	if !exists {
		return fmt.Errorf("resource type %s not found", resourceType)
	}

	resource, exists := resourceTypeMap[resourceID]
	if !exists {
		return fmt.Errorf("resource %s of type %s not found", resourceID, resourceType)
	}

	if resource.Allocated {
		return fmt.Errorf("resource %s of type %s is already allocated to %s", resourceID, resourceType, resource.AllocatedTo)
	}

	resource.Allocated = true
	resource.AllocatedTo = "system" // This could be enhanced to track actual allocator
	resource.AllocatedAt = time.Now()
	resource.LastUsed = time.Now()

	rm.logger.Info("Resource allocated", "resource_type", resourceType, "resource_id", resourceID, "allocated_to", resource.AllocatedTo)
	return nil
}

// Release releases a hardware resource
func (rm *ResourceManager) Release(resourceType string, resourceID string) error {
	rm.resourcesLock.Lock()
	defer rm.resourcesLock.Unlock()

	resourceTypeMap, exists := rm.resources[resourceType]
	if !exists {
		return fmt.Errorf("resource type %s not found", resourceType)
	}

	resource, exists := resourceTypeMap[resourceID]
	if !exists {
		return fmt.Errorf("resource %s of type %s not found", resourceID, resourceType)
	}

	if !resource.Allocated {
		return fmt.Errorf("resource %s of type %s is not allocated", resourceID, resourceType)
	}

	allocatedTo := resource.AllocatedTo
	resource.Allocated = false
	resource.AllocatedTo = ""
	resource.LastUsed = time.Now()

	rm.logger.Info("Resource released", "resource_type", resourceType, "resource_id", resourceID, "was_allocated_to", allocatedTo)
	return nil
}

// GetResource returns a resource by type and ID
func (rm *ResourceManager) GetResource(resourceType string, resourceID string) (*Resource, error) {
	rm.resourcesLock.RLock()
	defer rm.resourcesLock.RUnlock()

	resourceTypeMap, exists := rm.resources[resourceType]
	if !exists {
		return nil, fmt.Errorf("resource type %s not found", resourceType)
	}

	resource, exists := resourceTypeMap[resourceID]
	if !exists {
		return nil, fmt.Errorf("resource %s of type %s not found", resourceID, resourceType)
	}

	return resource, nil
}

// GetResourcesByType returns all resources of a specific type
func (rm *ResourceManager) GetResourcesByType(resourceType string) map[string]*Resource {
	rm.resourcesLock.RLock()
	defer rm.resourcesLock.RUnlock()

	resourceTypeMap, exists := rm.resources[resourceType]
	if !exists {
		return make(map[string]*Resource)
	}

	// Return a copy to avoid race conditions
	result := make(map[string]*Resource)
	for id, resource := range resourceTypeMap {
		result[id] = resource
	}

	return result
}

// GetAllResources returns all resources
func (rm *ResourceManager) GetAllResources() map[string]map[string]*Resource {
	rm.resourcesLock.RLock()
	defer rm.resourcesLock.RUnlock()

	result := make(map[string]map[string]*Resource)
	for resourceType, resources := range rm.resources {
		result[resourceType] = make(map[string]*Resource)
		for id, resource := range resources {
			result[resourceType][id] = resource
		}
	}

	return result
}

// IsResourceAllocated checks if a resource is allocated
func (rm *ResourceManager) IsResourceAllocated(resourceType string, resourceID string) bool {
	resource, err := rm.GetResource(resourceType, resourceID)
	if err != nil {
		return false
	}
	return resource.Allocated
}

// GetAvailableResources returns available (unallocated) resources of a type
func (rm *ResourceManager) GetAvailableResources(resourceType string) []string {
	rm.resourcesLock.RLock()
	defer rm.resourcesLock.RUnlock()

	var available []string

	resourceTypeMap, exists := rm.resources[resourceType]
	if !exists {
		return available
	}

	for resourceID, resource := range resourceTypeMap {
		if !resource.Allocated {
			available = append(available, resourceID)
		}
	}

	return available
}

// GetAllocatedResources returns allocated resources of a type
func (rm *ResourceManager) GetAllocatedResources(resourceType string) map[string]string {
	rm.resourcesLock.RLock()
	defer rm.resourcesLock.RUnlock()

	allocated := make(map[string]string)

	resourceTypeMap, exists := rm.resources[resourceType]
	if !exists {
		return allocated
	}

	for resourceID, resource := range resourceTypeMap {
		if resource.Allocated {
			allocated[resourceID] = resource.AllocatedTo
		}
	}

	return allocated
}

// AllocateForTask allocates resources needed for a task
func (rm *ResourceManager) AllocateForTask(taskID string, requiredResources map[string][]string) error {
	rm.logger.Info("Allocating resources for task", "task_id", taskID, "required_resources", requiredResources)

	// First, check if all resources are available
	for resourceType, resourceIDs := range requiredResources {
		for _, resourceID := range resourceIDs {
			if rm.IsResourceAllocated(resourceType, resourceID) {
				return fmt.Errorf("resource %s of type %s is already allocated", resourceID, resourceType)
			}
		}
	}

	// Allocate all resources
	for resourceType, resourceIDs := range requiredResources {
		for _, resourceID := range resourceIDs {
			if err := rm.Allocate(resourceType, resourceID); err != nil {
				// Rollback: release already allocated resources
				rm.rollbackAllocation(taskID, requiredResources, resourceType, resourceID)
				return fmt.Errorf("failed to allocate resource %s of type %s: %w", resourceID, resourceType, err)
			}
		}
	}

	rm.logger.Info("Resources allocated successfully for task", "task_id", taskID)
	return nil
}

// ReleaseForTask releases resources allocated for a task
func (rm *ResourceManager) ReleaseForTask(taskID string, requiredResources map[string][]string) error {
	rm.logger.Info("Releasing resources for task", "task_id", taskID)

	var errs []error

	for resourceType, resourceIDs := range requiredResources {
		for _, resourceID := range resourceIDs {
			if err := rm.Release(resourceType, resourceID); err != nil {
				rm.logger.Error("Failed to release resource", "resource_type", resourceType, "resource_id", resourceID, "error", err)
				errs = append(errs, err)
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors releasing resources for task %s: %v", taskID, errs)
	}

	rm.logger.Info("Resources released successfully for task", "task_id", taskID)
	return nil
}

// rollbackAllocation releases resources allocated during a failed allocation
func (rm *ResourceManager) rollbackAllocation(taskID string, requiredResources map[string][]string, failedResourceType string, failedResourceID string) {
	rm.logger.Warn("Rolling back resource allocation", "task_id", taskID, "failed_resource_type", failedResourceType, "failed_resource_id", failedResourceID)

	for resourceType, resourceIDs := range requiredResources {
		for _, resourceID := range resourceIDs {
			// Stop if we reach the failed resource
			if resourceType == failedResourceType && resourceID == failedResourceID {
				break
			}

			if err := rm.Release(resourceType, resourceID); err != nil {
				rm.logger.Error("Failed to rollback resource allocation", "resource_type", resourceType, "resource_id", resourceID, "error", err)
			}
		}
	}
}