// Package types defines the fundamental data structures and type definitions used throughout
// the motion control system. It includes task definitions, command structures, device configurations,
// and system state representations that form the common language between system components.
package types

import (
	"context"
	"time"
)

type TaskPriority int

const (
	PriorityEmergency TaskPriority = iota
	PriorityHigh
	PriorityMedium
	PriorityLow
)

type TaskStatus int

const (
	StatusPending TaskStatus = iota
	StatusRunning
	StatusPaused
	StatusCompleted
	StatusFailed
	StatusCancelled
)

type CommandType string

const (
	// Low-level CommandType implementations
	CommandMoveTo       CommandType = "move_to"
	CommandMoveRelative CommandType = "move_relative"
	CommandHome         CommandType = "home"
	CommandStop         CommandType = "stop"
	CommandJog          CommandType = "jog"
	CommandIO           CommandType = "io"
	CommandMotorControl CommandType = "motor_control"
	CommandMotorStatus  CommandType = "motor_status"
	CommandDelay       CommandType = "delay"
	CommandSequence    CommandType = "sequence"
	CommandParallel    CommandType = "parallel"
	CommandCondition   CommandType = "condition"
)


type AxisID string
type DeviceID string

type Point struct {
	X float64
	Y float64
	Z float64
}

type Velocity struct {
	Linear  float64
	Angular float64
}

type TaskNode interface {
	GetID() string
	GetType() CommandType
	GetParameters() map[string]interface{}
	GetStatus() TaskStatus
	GetChildren() []TaskNode
	GetParent() TaskNode
	AddChild(node TaskNode) error
	RemoveChild(nodeID string) error
	Validate() error
	Execute(ctx context.Context) error
	Cancel() error
	GetStartTime() time.Time
	GetEndTime() *time.Time
	GetDuration() time.Duration
	SetStatus(status TaskStatus)
	GetMetadata() map[string]interface{}
}

type IOCommand struct {
	DeviceID  DeviceID
	Channel   string
	Action    string // "set", "read", "toggle"
	Value     float64
	Threshold float64
	Timeout   time.Duration
}

type MotorCommand struct {
	DeviceID     DeviceID
	MotorID      string
	Action       string // "start", "stop", "set_speed", "set_position", "get_status"
	Speed        float64
	Position     Point
	Acceleration float64
	Torque       float64
	Direction    int // 1 for forward, -1 for reverse
	Enable       bool
}

type MotorStatus struct {
	DeviceID      DeviceID
	MotorID       string
	IsRunning     bool
	CurrentSpeed  float64
	CurrentPos    Point
	CurrentTorque float64
	Temperature   float64
	Error         string
	ErrorCode     int
}

type DelayCommand struct {
	Duration time.Duration
	Unit     string // "ms", "s", "min"
}

type SequenceCommand struct {
	Nodes        []TaskNode
	Mode         string // "sequential", "parallel"
	StopOnError bool
}

type ConditionCommand struct {
	Condition    string // "position_reached", "speed_achieved", "io_state", "timeout"
	DeviceID     DeviceID
	Channel      string
	TargetValue  float64
	Tolerance    float64
	Timeout      time.Duration
	PollInterval time.Duration
}

type Task struct {
	ID            string
	Type          CommandType
	Priority      TaskPriority
	Status        TaskStatus
	RootNode      TaskNode
	Target        Point
	Velocity      Velocity
	// Remove: Axes []AxisID - replaced with abstract fields
	DeviceGroup   string                `json:"device_group"`   // Device group identifier
	MotionSpace   MotionSpaceConfig     `json:"motion_space"`   // Motion space configuration
	Dimensions    []string              `json:"dimensions"`     // Target dimensions to move
	WorkSpace     string                `json:"workspace"`      // Workspace identifier
	Parameters    map[string]interface{} `json:"parameters"`
	CreatedAt     time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
	Error         string
	Timeout       time.Duration
	CancelFunc    context.CancelFunc
}

type MotionCommand struct {
	ID          string
	DeviceID    DeviceID
	CommandType CommandType
	Position    Point
	Velocity    Velocity
	Acceleration Velocity
	Data        []byte
	Timestamp   time.Time
	Timeout     time.Duration
}

type DeviceStatus struct {
	ID          DeviceID
	Connected   bool
	Error       error
	LastSeen    time.Time
	Position    Point
	Velocity    Velocity
	Temperature float64
	Voltage     float64
}

type IPCMessage struct {
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Target    string                 `json:"target"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	ID        string                 `json:"id"`
}

type SystemConfig struct {
	EventLoopInterval time.Duration                `yaml:"event_loop_interval"`
	QueueSize         int                          `yaml:"queue_size"`
	Devices           map[DeviceID]DeviceConfig    `yaml:"devices"`
	DeviceGroups      map[string]DeviceGroupConfig `yaml:"device_groups"`
	Safety            SafetyConfig                 `yaml:"safety"`
	IPC               IPCConfig                    `yaml:"ipc"`
	TaskTemplates     map[string]TaskTemplate      `yaml:"task_templates"`
	CommandMappings   map[string]CommandMapping `yaml:"command_mappings"`
}

type DeviceConfig struct {
	Type       string            `yaml:"type"`
	Protocol   string            `yaml:"protocol"`
	Endpoint   string            `yaml:"endpoint"`
	Parameters map[string]string `yaml:"parameters"`
	Timeout    time.Duration     `yaml:"timeout"`
	RetryCount int               `yaml:"retry_count"`
}

// DeviceGroupConfig represents a group of devices that work together
type DeviceGroupConfig struct {
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description"`
	DeviceIDs   []DeviceID                `yaml:"device_ids"`
	Dimensions  map[string]DimensionConfig `yaml:"dimensions"`
	Kinematics  KinematicsConfig          `yaml:"kinematics"`
	Capabilities map[string]interface{}   `yaml:"capabilities"`
}

// DimensionConfig defines a single dimension of motion
type DimensionConfig struct {
	Name         string  `yaml:"name"`         // Dimension name (e.g., "X", "Y", "Z", "Theta")
	Type         string  `yaml:"type"`         // "linear", "angular", "rotary"
	MinValue     float64 `yaml:"min_value"`    // Minimum value
	MaxValue     float64 `yaml:"max_value"`    // Maximum value
	HomeValue    float64 `yaml:"home_value"`   // Home/reference value
	MaxVelocity  float64 `yaml:"max_velocity"` // Maximum velocity
	MaxAccel     float64 `yaml:"max_accel"`    // Maximum acceleration
	Units        string  `yaml:"units"`        // Units (mm, deg, rad, etc.)
	Invert       bool    `yaml:"invert"`       // Invert direction
}

// KinematicsConfig defines the kinematic properties of a device group
type KinematicsConfig struct {
	Type         string                 `yaml:"type"`         // "cartesian", "cylindrical", "joint", "custom"
	Workspace    WorkspaceConfig        `yaml:"workspace"`    // Workspace definition
	Transforms   map[string]interface{} `yaml:"transforms"`   // Coordinate transforms
	Constraints  map[string]interface{} `yaml:"constraints"`  // Motion constraints
}

// WorkspaceConfig defines the workspace boundaries
type WorkspaceConfig struct {
	Shape       string                 `yaml:"shape"`       // "box", "cylinder", "sphere", "custom"
	Dimensions  map[string]float64     `yaml:"dimensions"`  // Shape dimensions
	Constraints map[string]interface{} `yaml:"constraints"` // Additional constraints
}

// MotionSpaceConfig defines the abstract motion space for tasks
type MotionSpaceConfig struct {
	SpaceType    string                    `yaml:"space_type"`    // Motion space type
	Dimensions   []DimensionConfig         `yaml:"dimensions"`    // Dimension configurations
	Constraints  map[string]interface{}    `yaml:"constraints"`   // Motion constraints
	Coordinates  CoordinateSystemConfig    `yaml:"coordinates"`   // Coordinate system
}

// CoordinateSystemConfig defines the coordinate system
type CoordinateSystemConfig struct {
	Type        string                 `yaml:"type"`        // "world", "tool", "joint", "custom"
	Reference   string                 `yaml:"reference"`   // Reference frame
	Orientation map[string]interface{} `yaml:"orientation"` // Orientation definition
}

// Legacy AxisConfig for backward compatibility
type AxisConfig struct {
	DeviceID         DeviceID  `yaml:"device_id"`
	MaxVelocity      float64   `yaml:"max_velocity"`
	MaxAcceleration  float64   `yaml:"max_acceleration"`
	MinPosition      float64   `yaml:"min_position"`
	MaxPosition      float64   `yaml:"max_position"`
	HomePosition     float64   `yaml:"home_position"`
	Units            string    `yaml:"units"`
	InvertDirection  bool      `yaml:"invert_direction"`
}

type SafetyConfig struct {
	EnableLimits      bool      `yaml:"enable_limits"`
	EnableEmergency   bool      `yaml:"enable_emergency"`
	MaxTemperature    float64   `yaml:"max_temperature"`
	MinVoltage        float64   `yaml:"min_voltage"`
	WatchdogTimeout   time.Duration `yaml:"watchdog_timeout"`
}

type IPCConfig struct {
	Type       string        `yaml:"type"`
	Address    string        `yaml:"address"`
	Port       int           `yaml:"port"`
	Timeout    time.Duration `yaml:"timeout"`
	BufferSize int           `yaml:"buffer_size"`
}

type TaskTemplate struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Type        CommandType            `yaml:"type"`
	Parameters  map[string]interface{} `yaml:"parameters"`
	Nodes       []TaskNodeConfig       `yaml:"nodes,omitempty"`
	Timeout     time.Duration          `yaml:"timeout"`
	Priority    TaskPriority           `yaml:"priority"`
}

type TaskNodeConfig struct {
	ID         string                 `yaml:"id"`
	Type       CommandType            `yaml:"type"`
	Parameters map[string]interface{} `yaml:"parameters"`
	Children   []TaskNodeConfig       `yaml:"children,omitempty"`
}

type CommandMapping struct {
	CommandName string                 `yaml:"command_name"`
	Description string                 `yaml:"description"`
	Template    string                 `yaml:"template,omitempty"`
	Nodes       []TaskNodeConfig       `yaml:"nodes,omitempty"`
	Parameters  map[string]interface{} `yaml:"parameters,omitempty"`
	Priority    TaskPriority           `yaml:"priority"`
	Timeout     time.Duration          `yaml:"timeout"`
}

// BusinessCommand represents high-level business commands for frontend interaction
// These commands are business-oriented and implementation-agnostic
type BusinessCommand string

const (
	// System Control Commands
	CmdQueryStatus     BusinessCommand = "query"      // Query system status
	CmdInitialize      BusinessCommand = "initialize" // Initialize system
	CmdEmergencyStop   BusinessCommand = "stop"       // Emergency stop
	CmdReset           BusinessCommand = "reset"      // Reset system

	// Motion Control Commands
	CmdMove            BusinessCommand = "move"       // Move to position
	CmdMoveRelative    BusinessCommand = "move_relative" // Move relative to current position
	CmdHome            BusinessCommand = "home"       // Home axes
	CmdJog             BusinessCommand = "jog"        // Manual jog operation
	CmdStopMotion      BusinessCommand = "stop_motion" // Stop motion

	// Safety and Check Commands
	CmdSafetyCheck     BusinessCommand = "safety"     // Safety check
	CmdSelfCheck       BusinessCommand = "self_check" // System self-check

	// Configuration Commands
	CmdGetConfig       BusinessCommand = "get_config" // Get configuration
	CmdSetConfig       BusinessCommand = "set_config" // Set configuration
	CmdListTemplates   BusinessCommand = "list_templates" // List task templates
)

// BusinessMessage represents simplified frontend command message
type BusinessMessage struct {
	Command   BusinessCommand       `json:"command"`    // Business command type
	Params    map[string]interface{} `json:"params"`     // Command parameters
	RequestID string                `json:"request_id"` // Request identifier
	Source    string                `json:"source"`     // Message source
	Target    string                `json:"target"`     // Message target
	Timestamp time.Time              `json:"timestamp"`  // Message timestamp
}

// BusinessResponse represents response to business command
type BusinessResponse struct {
	RequestID string                 `json:"request_id"` // Original request ID
	Status    string                 `json:"status"`     // Response status
	Data      map[string]interface{} `json:"data"`       // Response data
	Error     string                 `json:"error"`      // Error message if any
	Timestamp time.Time              `json:"timestamp"`  // Response timestamp
}

// Command parameters structures
type MoveParams struct {
	Target    Point   `json:"target"`     // Target position
	Velocity  Velocity `json:"velocity"`   // Target velocity
	Axes      []AxisID `json:"axes"`       // Axes to move
	Mode      string  `json:"mode"`       // Move mode (precise, rapid, etc.)
	Timeout   time.Duration `json:"timeout"`  // Operation timeout
}

type HomeParams struct {
	Axes     []AxisID `json:"axes"`   // Axes to home
	Sequence []string `json:"sequence"` // Home sequence
	Mode     string   `json:"mode"`   // Home mode
	Timeout  time.Duration `json:"timeout"`  // Operation timeout
}

type SafetyCheckParams struct {
	CheckType string                 `json:"check_type"` // Type of safety check
	Params    map[string]interface{} `json:"params"`     // Check-specific parameters
	Timeout   time.Duration          `json:"timeout"`    // Check timeout
}

type ConfigParams struct {
	ConfigType string                 `json:"config_type"` // Configuration type
	Params     map[string]interface{} `json:"params"`      // Configuration parameters
}