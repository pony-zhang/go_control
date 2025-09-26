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

// High-level abstract commands for client interaction
type AbstractCommand string

const (
	AbstractSelfCheck     AbstractCommand = "self_check"     // 自检
	AbstractReset         AbstractCommand = "reset"         // 复位
	AbstractStart         AbstractCommand = "start"         // 启动
	AbstractStop          AbstractCommand = "stop"          // 停止
	AbstractEmergencyStop AbstractCommand = "emergency_stop" // 急停
	AbstractHome          AbstractCommand = "home"          // 回零
	AbstractInitialize    AbstractCommand = "initialize"    // 初始化
	AbstractPause         AbstractCommand = "pause"         // 暂停
	AbstractResume        AbstractCommand = "resume"        // 恢复
	AbstractSafetyCheck   AbstractCommand = "safety_check"  // 安全检查
	AbstractReady         AbstractCommand = "ready"         // 准备就绪
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
	ID          string
	Type        CommandType
	Priority    TaskPriority
	Status      TaskStatus
	RootNode    TaskNode
	Target      Point
	Velocity    Velocity
	Axes        []AxisID
	Parameters  map[string]interface{}
	CreatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
	Error       string
	Timeout     time.Duration
	CancelFunc  context.CancelFunc
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
	Axes              map[AxisID]AxisConfig        `yaml:"axes"`
	Safety            SafetyConfig                 `yaml:"safety"`
	IPC               IPCConfig                    `yaml:"ipc"`
	TaskTemplates     map[string]TaskTemplate      `yaml:"task_templates"`
	CommandMappings   map[AbstractCommand]CommandMapping `yaml:"command_mappings"`
}

type DeviceConfig struct {
	Type       string            `yaml:"type"`
	Protocol   string            `yaml:"protocol"`
	Endpoint   string            `yaml:"endpoint"`
	Parameters map[string]string `yaml:"parameters"`
	Timeout    time.Duration     `yaml:"timeout"`
	RetryCount int               `yaml:"retry_count"`
}

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
	AbstractCommand AbstractCommand         `yaml:"abstract_command"`
	Description     string                 `yaml:"description"`
	Template        string                 `yaml:"template,omitempty"`
	Nodes           []TaskNodeConfig       `yaml:"nodes,omitempty"`
	Parameters      map[string]interface{} `yaml:"parameters,omitempty"`
	Priority        TaskPriority           `yaml:"priority"`
	Timeout         time.Duration          `yaml:"timeout"`
}