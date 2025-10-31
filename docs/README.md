# Motion Control System Documentation

## Overview

This directory contains comprehensive technical documentation for the motion control system, covering architecture, data flow, and development guidelines.

## Documentation Structure

### 📁 [architecture.md](./architecture.md)
**System Architecture**

Four-layer modular architecture with event-driven command processing:
- Coordination Layer (Event Loop + Management)
- Infrastructure Layer (Config + Device + IPC)
- Application Layer (Business → Task → Service → HAL)
- Hardware Abstraction Layer (HAL)

### 📁 [development.md](./development.md)
**Development Guide**

Comprehensive development guide including:
- Development environment setup
- Event-driven command processing
- HAL protocol implementation
- Testing strategies
- Deployment guidelines

### 📁 [data-flow.md](./data-flow.md)
**Data Flow and State Management**

Event-driven data flow analysis:
- Command routing mechanisms
- Layer communication patterns
- State management
- Error handling
- Performance monitoring

## Quick Start

### 🎯 For New Users
1. Read [architecture.md](./architecture.md) to understand the system design
2. Follow the development guide in [development.md](./development.md)
3. Check the data flow documentation in [data-flow.md](./data-flow.md)

### 🔧 For Developers
1. **Adding New Commands**: Implement `CommandHandler` interface
2. **Adding New Protocols**: Use HAL `Protocol` interface
3. **Adding New Layers**: Follow the horizontal layer pattern
4. **Testing**: Run `go test ./...` before submitting

### 🐛 Troubleshooting
1. Check [data-flow.md](./data-flow.md) for error handling patterns
2. Use structured logging for debugging
3. Monitor system state through IPC interface

## Key Concepts

### Event-Driven Architecture
- **CommandRouter**: Intelligent routing of business commands
- **CommandHandler**: Modular command processing interface
- **BusinessCommand**: High-level abstract command types
- **Subscription-based**: Modules subscribe to commands they handle

### Hardware Abstraction Layer (HAL)
- **Unified Interface**: Single interface for diverse hardware
- **Multi-Protocol Support**: Mock, Modbus, Serial, Custom protocols
- **Resource Management**: Dynamic hardware resource allocation
- **Protocol Adapters**: Extensible protocol framework

### Four-Layer Architecture
- **Coordination**: Event-driven system coordination
- **Infrastructure**: Core services (config, devices, IPC)
- **Application**: Business logic, task orchestration, service coordination
- **Physical**: Hardware devices and sensors

## Getting Help

### 📧 Support
- **Issues**: Use GitHub Issues with appropriate labels
- **Discussions**: Use GitHub Discussions for questions
- **Documentation**: Submit PRs for documentation improvements

### 📚 External Resources
- [Go Documentation](https://golang.org/doc/)
- [HAL Design Patterns](https://en.wikipedia.org/wiki/Hardware_abstraction)
- [Industrial Automation Standards](https://www.iso.org/committee/539028.html)

---

*Last updated: 2025-10-31*
*Current version: v3.0.0 - Event-Driven Command Processing Architecture*