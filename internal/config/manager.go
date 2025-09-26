package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"control/pkg/types"
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
}

func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath:   configPath,
		watchers:     make([]func(types.SystemConfig), 0),
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

	log.Printf("Configuration loaded from %s", cm.configPath)
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
	log.Printf("Configuration updated and saved to %s", cm.configPath)
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

	log.Printf("Started watching config file: %s", cm.configPath)
	return nil
}

func (cm *ConfigManager) StopWatching() error {
	if !cm.watching {
		return fmt.Errorf("config watcher is not running")
	}

	cm.cancel()
	cm.wg.Wait()
	cm.watching = false

	log.Println("Stopped watching config file")
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
			log.Printf("Error checking config file: %v", err)
		}
		return
	}

	if info.ModTime().After(cm.lastModified) {
		log.Printf("Config file modified, reloading...")
		if err := cm.Reload(); err != nil {
			log.Printf("Failed to reload config: %v", err)
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
		config.CommandMappings = make(map[types.AbstractCommand]types.CommandMapping)
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

	for axisID, axisConfig := range config.Axes {
		if axisConfig.DeviceID == "" {
			return fmt.Errorf("axis %s must have a device ID", axisID)
		}
		if axisConfig.MaxVelocity <= 0 {
			return fmt.Errorf("axis %s must have a positive max velocity", axisID)
		}
		if axisConfig.MaxAcceleration <= 0 {
			return fmt.Errorf("axis %s must have a positive max acceleration", axisID)
		}
		if axisConfig.MinPosition >= axisConfig.MaxPosition {
			return fmt.Errorf("axis %s must have min position < max position", axisID)
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

func (cm *ConfigManager) GetAxisConfig(axisID types.AxisID) (types.AxisConfig, error) {
	cm.configLock.RLock()
	defer cm.configLock.RUnlock()

	config, exists := cm.config.Axes[axisID]
	if !exists {
		return types.AxisConfig{}, fmt.Errorf("axis %s not found in configuration", axisID)
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

func (cm *ConfigManager) UpdateAxisConfig(axisID types.AxisID, config types.AxisConfig) error {
	cm.configLock.Lock()
	defer cm.configLock.Unlock()

	if _, exists := cm.config.Axes[axisID]; !exists {
		return fmt.Errorf("axis %s not found in configuration", axisID)
	}

	cm.config.Axes[axisID] = config
	return cm.SetConfig(cm.config)
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
		CommandMappings: map[types.AbstractCommand]types.CommandMapping{
			types.AbstractSelfCheck: {
				AbstractCommand: types.AbstractSelfCheck,
				Description:     "执行系统自检",
				Template:        "io_check",
				Priority:        types.PriorityHigh,
				Timeout:         10 * time.Second,
			},
			types.AbstractReset: {
				AbstractCommand: types.AbstractReset,
				Description:     "复位系统到初始状态",
				Nodes: []types.TaskNodeConfig{
					{
						ID:   "stop-all",
						Type: types.CommandMotorControl,
						Parameters: map[string]interface{}{
							"device_id": "device-1",
							"motor_id":  "all",
							"action":    "stop",
						},
					},
					{
						ID:   "reset-delay",
						Type: types.CommandDelay,
						Parameters: map[string]interface{}{
							"duration": 2 * time.Second,
							"unit":     "s",
						},
					},
				},
				Priority: types.PriorityHigh,
				Timeout:  15 * time.Second,
			},
			types.AbstractStart: {
				AbstractCommand: types.AbstractStart,
				Description:     "启动系统",
				Template:        "safety_check",
				Priority:        types.PriorityMedium,
				Timeout:         20 * time.Second,
			},
			types.AbstractStop: {
				AbstractCommand: types.AbstractStop,
				Description:     "停止系统",
				Nodes: []types.TaskNodeConfig{
					{
						ID:   "stop-motors",
						Type: types.CommandMotorControl,
						Parameters: map[string]interface{}{
							"device_id": "device-1",
							"motor_id":  "all",
							"action":    "stop",
						},
					},
				},
				Priority: types.PriorityHigh,
				Timeout:  10 * time.Second,
			},
			types.AbstractEmergencyStop: {
				AbstractCommand: types.AbstractEmergencyStop,
				Description:     "急停系统",
				Nodes: []types.TaskNodeConfig{
					{
						ID:   "emergency-stop-motors",
						Type: types.CommandMotorControl,
						Parameters: map[string]interface{}{
							"device_id": "device-1",
							"motor_id":  "all",
							"action":    "stop",
						},
					},
				},
				Priority: types.PriorityEmergency,
				Timeout:  5 * time.Second,
			},
			types.AbstractHome: {
				AbstractCommand: types.AbstractHome,
				Description:     "回零所有轴",
				Template:        "home_all",
				Priority:        types.PriorityHigh,
				Timeout:         30 * time.Second,
			},
			types.AbstractInitialize: {
				AbstractCommand: types.AbstractInitialize,
				Description:     "初始化系统",
				Nodes: []types.TaskNodeConfig{
					{
						ID:   "init-self-check",
						Type: types.CommandSequence,
						Parameters: map[string]interface{}{
							"mode":         "sequential",
							"stop_on_error": true,
						},
						Children: []types.TaskNodeConfig{
							{
								ID:   "init-io-check",
								Type: types.CommandIO,
								Parameters: map[string]interface{}{
									"device_id": "device-1",
									"channel":   "system-status",
									"action":    "read",
								},
							},
							{
								ID:   "init-delay",
								Type: types.CommandDelay,
								Parameters: map[string]interface{}{
									"duration": 1 * time.Second,
									"unit":     "s",
								},
							},
						},
					},
					{
						ID:   "init-home",
						Type: types.CommandSequence,
						Parameters: map[string]interface{}{
							"mode":         "sequential",
							"stop_on_error": true,
						},
						Children: []types.TaskNodeConfig{
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
					},
				},
				Priority: types.PriorityHigh,
				Timeout:  60 * time.Second,
			},
			types.AbstractSafetyCheck: {
				AbstractCommand: types.AbstractSafetyCheck,
				Description:     "安全检查",
				Template:        "safety_check",
				Priority:        types.PriorityHigh,
				Timeout:         15 * time.Second,
			},
		},
		Devices: map[types.DeviceID]types.DeviceConfig{
			"device-1": {
				Type:     "controller",
				Protocol: "mock",
				Endpoint: "mock://device-1",
				Timeout:  10 * time.Second,
			},
		},
		Axes: map[types.AxisID]types.AxisConfig{
			"axis-x": {
				DeviceID:        "device-1",
				MaxVelocity:     100.0,
				MaxAcceleration: 50.0,
				MinPosition:     -1000.0,
				MaxPosition:     1000.0,
				HomePosition:    0.0,
				Units:           "mm",
			},
			"axis-y": {
				DeviceID:        "device-1",
				MaxVelocity:     100.0,
				MaxAcceleration: 50.0,
				MinPosition:     -1000.0,
				MaxPosition:     1000.0,
				HomePosition:    0.0,
				Units:           "mm",
			},
			"axis-z": {
				DeviceID:        "device-1",
				MaxVelocity:     50.0,
				MaxAcceleration: 25.0,
				MinPosition:     -500.0,
				MaxPosition:     500.0,
				HomePosition:    0.0,
				Units:           "mm",
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

	log.Printf("Configuration exported to %s", path)
	return nil
}

func (cm *ConfigManager) ImportConfig(path string) error {
	if err := cm.LoadConfig(path); err != nil {
		return fmt.Errorf("failed to import config: %w", err)
	}

	if err := cm.SetConfig(cm.GetConfig()); err != nil {
		return fmt.Errorf("failed to save imported config: %w", err)
	}

	log.Printf("Configuration imported from %s", path)
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