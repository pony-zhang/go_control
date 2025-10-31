// Package config provides comprehensive YAML-based configuration management with
// hot-reload capabilities. It handles device configurations, axis parameters, safety limits,
// task templates, and system settings that can be updated at runtime without service interruption.
package config

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"control/pkg/types"
	"control/internal/logging"
)

type ConfigManager struct {
	config        types.SystemConfig
	configPath    string
	configLock    sync.RWMutex
	watchers      []func(types.SystemConfig)
	watchersLock  sync.RWMutex
	lastModified  time.Time
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	watching      bool
	logger        *logging.Logger
}

func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath:   configPath,
		watchers:     make([]func(types.SystemConfig), 0),
		logger:       logging.GetLogger("config_manager"),
	}
}

func (cm *ConfigManager) LoadConfig(path string) error {
	if path != "" {
		cm.configPath = path
	}

	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config types.SystemConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cm.validateConfig(&config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	cm.config = config
	cm.lastModified = time.Now()

	cm.logger.Info("Configuration loaded", "config_path", cm.configPath)
	return nil
}

func (cm *ConfigManager) Reload() error {
	return cm.LoadConfig(cm.configPath)
}

func (cm *ConfigManager) GetConfig() types.SystemConfig {
	cm.configLock.RLock()
	defer cm.configLock.RUnlock()
	return cm.config
}

func (cm *ConfigManager) SetConfig(config types.SystemConfig) error {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	if err := cm.validateConfig(&config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(cm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	cm.config = config
	cm.lastModified = time.Now()

	cm.notifyWatchers()
	cm.logger.Info("Configuration updated and saved", "config_path", cm.configPath)
	return nil
}

func (cm *ConfigManager) WatchChanges(callback func(types.SystemConfig)) error {
	cm.watchersLock.Lock()
	defer cm.watchersLock.Unlock()

	cm.watchers = append(cm.watchers, callback)
	return nil
}

func (cm *ConfigManager) StartWatching(ctx context.Context) error {
	if cm.watching {
		return fmt.Errorf("config watcher is already running")
	}

	cm.ctx, cm.cancel = context.WithCancel(ctx)
	cm.watching = true

	cm.wg.Add(1)
	go cm.watchFile()

	cm.logger.Info("Started watching config file", "config_path", cm.configPath)
	return nil
}

func (cm *ConfigManager) StopWatching() error {
	if !cm.watching {
		return fmt.Errorf("config watcher is not running")
	}

	cm.cancel()
	cm.wg.Wait()
	cm.watching = false

	cm.logger.Info("Stopped watching config file")
	return nil
}

func (cm *ConfigManager) watchFile() {
	defer cm.wg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.checkFileChanges()
		}
	}
}

func (cm *ConfigManager) checkFileChanges() {
	info, err := os.Stat(cm.configPath)
	if err != nil {
		if !os.IsNotExist(err) {
			cm.logger.Error("Error checking config file", "error", err)
		}
		return
	}

	if info.ModTime().After(cm.lastModified) {
		cm.logger.Info("Config file modified, reloading...")
		if err := cm.Reload(); err != nil {
			cm.logger.Error("Failed to reload config", "error", err)
		} else {
			cm.notifyWatchers()
		}
	}
}

func (cm *ConfigManager) notifyWatchers() {
	cm.watchersLock.RLock()
	watchers := make([]func(types.SystemConfig), len(cm.watchers))
	copy(watchers, cm.watchers)
	cm.watchersLock.RUnlock()

	config := cm.GetConfig()
	for _, watcher := range watchers {
		go watcher(config)
	}
}

func (cm *ConfigManager) validateConfig(config *types.SystemConfig) error {
	if config.EventLoopInterval <= 0 {
		config.EventLoopInterval = 10 * time.Millisecond
	}

	if config.QueueSize <= 0 {
		config.QueueSize = 1000
	}

	if config.IPC.BufferSize <= 0 {
		config.IPC.BufferSize = 1024
	}

	if config.IPC.Timeout <= 0 {
		config.IPC.Timeout = 5 * time.Second
	}

	// Initialize task templates if not provided
	if config.TaskTemplates == nil {
		config.TaskTemplates = make(map[string]types.TaskTemplate)
	}

	// Initialize command mappings if not provided
	if config.CommandMappings == nil {
		config.CommandMappings = make(map[string]types.CommandMapping)
	}

	// Initialize device groups if not provided
	if config.DeviceGroups == nil {
		config.DeviceGroups = make(map[string]types.DeviceGroupConfig)
	}

	if len(config.Devices) == 0 {
		return fmt.Errorf("at least one device must be configured")
	}

	for deviceID, deviceConfig := range config.Devices {
		if deviceConfig.Type == "" {
			return fmt.Errorf("device %s must have a type", deviceID)
		}
		if deviceConfig.Protocol == "" {
			return fmt.Errorf("device %s must have a protocol", deviceID)
		}
		if deviceConfig.Endpoint == "" {
			return fmt.Errorf("device %s must have an endpoint", deviceID)
		}
	}

	// Validate device groups
	for groupID, groupConfig := range config.DeviceGroups {
		if groupConfig.Name == "" {
			return fmt.Errorf("device group %s must have a name", groupID)
		}
		if len(groupConfig.DeviceIDs) == 0 {
			return fmt.Errorf("device group %s must have at least one device", groupID)
		}
		// Check all devices in the group exist
		for _, deviceID := range groupConfig.DeviceIDs {
			if _, exists := config.Devices[deviceID]; !exists {
				return fmt.Errorf("device %s in group %s does not exist", deviceID, groupID)
			}
		}
		// Validate dimensions
		for dimName, dimConfig := range groupConfig.Dimensions {
			if dimConfig.Name == "" {
				return fmt.Errorf("dimension %s in group %s must have a name", dimName, groupID)
			}
			if dimConfig.Type == "" {
				return fmt.Errorf("dimension %s in group %s must have a type", dimName, groupID)
			}
			if dimConfig.MaxVelocity <= 0 {
				return fmt.Errorf("dimension %s in group %s must have positive max velocity", dimName, groupID)
			}
			if dimConfig.MaxAccel <= 0 {
				return fmt.Errorf("dimension %s in group %s must have positive max acceleration", dimName, groupID)
			}
			if dimConfig.MinValue >= dimConfig.MaxValue {
				return fmt.Errorf("dimension %s in group %s must have min < max", dimName, groupID)
			}
		}
	}

	return nil
}

func (cm *ConfigManager) GetDeviceConfig(deviceID types.DeviceID) (types.DeviceConfig, error) {
	cm.configLock.RLock()
	defer cm.configLock.RUnlock()

	config, exists := cm.config.Devices[deviceID]
	if !exists {
		return types.DeviceConfig{}, fmt.Errorf("device %s not found in configuration", deviceID)
	}

	return config, nil
}

func (cm *ConfigManager) GetDeviceGroupConfig(groupID string) (types.DeviceGroupConfig, error) {
	cm.configLock.RLock()
	defer cm.configLock.RUnlock()

	config, exists := cm.config.DeviceGroups[groupID]
	if !exists {
		return types.DeviceGroupConfig{}, fmt.Errorf("device group %s not found in configuration", groupID)
	}

	return config, nil
}

func (cm *ConfigManager) UpdateDeviceConfig(deviceID types.DeviceID, config types.DeviceConfig) error {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	if _, exists := cm.config.Devices[deviceID]; !exists {
		return fmt.Errorf("device %s not found in configuration", deviceID)
	}

	cm.config.Devices[deviceID] = config
	return cm.SetConfig(cm.config)
}

func (cm *ConfigManager) UpdateDeviceGroupConfig(groupID string, config types.DeviceGroupConfig) error {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	if _, exists := cm.config.DeviceGroups[groupID]; !exists {
		return fmt.Errorf("device group %s not found in configuration", groupID)
	}

	cm.config.DeviceGroups[groupID] = config
	return cm.SetConfig(cm.config)
}

func (cm *ConfigManager) AddDeviceGroup(groupID string, config types.DeviceGroupConfig) error {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	if _, exists := cm.config.DeviceGroups[groupID]; exists {
		return fmt.Errorf("device group %s already exists", groupID)
	}

	cm.config.DeviceGroups[groupID] = config
	return cm.SetConfig(cm.config)
}

func (cm *ConfigManager) RemoveDeviceGroup(groupID string) error {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	if _, exists := cm.config.DeviceGroups[groupID]; !exists {
		return fmt.Errorf("device group %s not found in configuration", groupID)
	}

	delete(cm.config.DeviceGroups, groupID)
	return cm.SetConfig(cm.config)
}

func (cm *ConfigManager) GetDimensionConfig(groupID, dimensionName string) (types.DimensionConfig, error) {
	groupConfig, err := cm.GetDeviceGroupConfig(groupID)
	if err != nil {
		return types.DimensionConfig{}, err
	}

	dimension, exists := groupConfig.Dimensions[dimensionName]
	if !exists {
		return types.DimensionConfig{}, fmt.Errorf("dimension %s not found in device group %s", dimensionName, groupID)
	}

	return dimension, nil
}

func (cm *ConfigManager) ListDeviceGroups() []string {
	cm.configLock.RLock()
	defer cm.configLock.RUnlock()

	groups := make([]string, 0, len(cm.config.DeviceGroups))
	for groupID := range cm.config.DeviceGroups {
		groups = append(groups, groupID)
	}

	return groups
}

func (cm *ConfigManager) CreateDefaultConfig() error {
	defaultConfig := types.SystemConfig{
		EventLoopInterval: 10 * time.Millisecond,
		QueueSize:         1000,
		IPC: types.IPCConfig{
			Type:       "tcp",
			Address:    "127.0.0.1",
			Port:       8080,
			Timeout:    5 * time.Second,
			BufferSize: 1024,
		},
		TaskTemplates: map[string]types.TaskTemplate{
			"home_all": {
				Name:        "Home All Axes",
				Description: "Home all axes to their home positions",
				Type:        types.CommandSequence,
				Parameters: map[string]interface{}{
					"mode":         "sequential",
					"stop_on_error": true,
				},
				Nodes: []types.TaskNodeConfig{
					{
						ID:   "home-x",
						Type: types.CommandHome,
						Parameters: map[string]interface{}{
							"axes": []string{"axis-x"},
						},
					},
					{
						ID:   "home-y",
						Type: types.CommandHome,
						Parameters: map[string]interface{}{
							"axes": []string{"axis-y"},
						},
					},
					{
						ID:   "home-z",
						Type: types.CommandHome,
						Parameters: map[string]interface{}{
							"axes": []string{"axis-z"},
						},
					},
				},
				Timeout:  30 * time.Second,
				Priority: types.PriorityHigh,
			},
			"io_check": {
				Name:        "IO Status Check",
				Description: "Check IO status before operation",
				Type:        types.CommandSequence,
				Parameters: map[string]interface{}{
					"mode":         "sequential",
					"stop_on_error": true,
				},
				Nodes: []types.TaskNodeConfig{
					{
						ID:   "check-sensor-1",
						Type: types.CommandIO,
						Parameters: map[string]interface{}{
							"device_id": "device-1",
							"channel":   "sensor-1",
							"action":    "read",
						},
					},
					{
						ID:   "check-sensor-2",
						Type: types.CommandIO,
						Parameters: map[string]interface{}{
							"device_id": "device-1",
							"channel":   "sensor-2",
							"action":    "read",
						},
					},
				},
				Timeout:  5 * time.Second,
				Priority: types.PriorityMedium,
			},
			"motor_start_sequence": {
				Name:        "Motor Start Sequence",
				Description: "Start motors with delay between each",
				Type:        types.CommandSequence,
				Parameters: map[string]interface{}{
					"mode":         "sequential",
					"stop_on_error": true,
				},
				Nodes: []types.TaskNodeConfig{
					{
						ID:   "start-motor-1",
						Type: types.CommandMotorControl,
						Parameters: map[string]interface{}{
							"device_id": "device-1",
							"motor_id":  "motor-1",
							"action":    "start",
							"speed":     100.0,
						},
					},
					{
						ID:   "delay-1",
						Type: types.CommandDelay,
						Parameters: map[string]interface{}{
							"duration": 2 * time.Second,
							"unit":     "s",
						},
					},
					{
						ID:   "start-motor-2",
						Type: types.CommandMotorControl,
						Parameters: map[string]interface{}{
							"device_id": "device-1",
							"motor_id":  "motor-2",
							"action":    "start",
							"speed":     150.0,
						},
					},
				},
				Timeout:  10 * time.Second,
				Priority: types.PriorityMedium,
			},
		},
		CommandMappings: make(map[string]types.CommandMapping),
		Devices: map[types.DeviceID]types.DeviceConfig{
			"device-1": {
				Type:     "controller",
				Protocol: "mock",
				Endpoint: "mock://device-1",
				Timeout:  10 * time.Second,
			},
		},
		DeviceGroups: map[string]types.DeviceGroupConfig{
			"main_workspace": {
				Name:        "Main Workspace",
				Description: "Primary 3-axis cartesian workspace",
				DeviceIDs:   []types.DeviceID{"device-1"},
				Dimensions: map[string]types.DimensionConfig{
					"X": {
						Name:        "X",
						Type:        "linear",
						MinValue:    -1000.0,
						MaxValue:    1000.0,
						HomeValue:   0.0,
						MaxVelocity: 100.0,
						MaxAccel:    50.0,
						Units:       "mm",
						Invert:      false,
					},
					"Y": {
						Name:        "Y",
						Type:        "linear",
						MinValue:    -1000.0,
						MaxValue:    1000.0,
						HomeValue:   0.0,
						MaxVelocity: 100.0,
						MaxAccel:    50.0,
						Units:       "mm",
						Invert:      false,
					},
					"Z": {
						Name:        "Z",
						Type:        "linear",
						MinValue:    -500.0,
						MaxValue:    500.0,
						HomeValue:   0.0,
						MaxVelocity: 50.0,
						MaxAccel:    25.0,
						Units:       "mm",
						Invert:      false,
					},
				},
				Kinematics: types.KinematicsConfig{
					Type: "cartesian",
					Workspace: types.WorkspaceConfig{
						Shape: "box",
						Dimensions: map[string]float64{
							"width":  2000.0,
							"depth":  2000.0,
							"height": 1000.0,
						},
						Constraints: map[string]interface{}{},
					},
					Transforms:  map[string]interface{}{},
					Constraints: map[string]interface{}{},
				},
				Capabilities: map[string]interface{}{
					"linear_motion": true,
					"home_all":      true,
					"emergency_stop": true,
				},
			},
		},
		Safety: types.SafetyConfig{
			EnableLimits:     true,
			EnableEmergency:  true,
			MaxTemperature:   80.0,
			MinVoltage:       12.0,
			WatchdogTimeout:  10 * time.Second,
		},
	}

	return cm.SetConfig(defaultConfig)
}

func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}

func (cm *ConfigManager) ExportConfig(path string) error {
	cm.configLock.RLock()
	defer cm.configLock.RUnlock()

	data, err := yaml.Marshal(cm.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}

	cm.logger.Info("Configuration exported", "path", path)
	return nil
}

func (cm *ConfigManager) ImportConfig(path string) error {
	if err := cm.LoadConfig(path); err != nil {
		return fmt.Errorf("failed to import config: %w", err)
	}

	if err := cm.SetConfig(cm.GetConfig()); err != nil {
		return fmt.Errorf("failed to save imported config: %w", err)
	}

	cm.logger.Info("Configuration imported", "path", path)
	return nil
}

func (cm *ConfigManager) GetTaskTemplate(name string) (types.TaskTemplate, error) {
	cm.configLock.RLock()
	defer cm.configLock.RUnlock()

	template, exists := cm.config.TaskTemplates[name]
	if !exists {
		return types.TaskTemplate{}, fmt.Errorf("task template '%s' not found", name)
	}

	return template, nil
}

func (cm *ConfigManager) AddTaskTemplate(name string, template types.TaskTemplate) error {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	if _, exists := cm.config.TaskTemplates[name]; exists {
		return fmt.Errorf("task template '%s' already exists", name)
	}

	cm.config.TaskTemplates[name] = template
	return cm.SetConfig(cm.config)
}

func (cm *ConfigManager) UpdateTaskTemplate(name string, template types.TaskTemplate) error {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	if _, exists := cm.config.TaskTemplates[name]; !exists {
		return fmt.Errorf("task template '%s' not found", name)
	}

	cm.config.TaskTemplates[name] = template
	return cm.SetConfig(cm.config)
}

func (cm *ConfigManager) RemoveTaskTemplate(name string) error {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	if _, exists := cm.config.TaskTemplates[name]; !exists {
		return fmt.Errorf("task template '%s' not found", name)
	}

	delete(cm.config.TaskTemplates, name)
	return cm.SetConfig(cm.config)
}

func (cm *ConfigManager) ListTaskTemplates() []string {
	cm.configLock.RLock()
	defer cm.configLock.RUnlock()

	templates := make([]string, 0, len(cm.config.TaskTemplates))
	for name := range cm.config.TaskTemplates {
		templates = append(templates, name)
	}

	return templates
}