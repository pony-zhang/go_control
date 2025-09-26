package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"control/pkg/types"
)

type IPCClient struct {
	config      types.IPCConfig
	conn        net.Conn
	decoder     *json.Decoder
	encoder     *json.Encoder
	receiveChan chan types.IPCMessage
	sendChan    chan []byte
	handlers    map[string]func(types.IPCMessage)
	handlersLock sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	connected   bool
}

func NewIPCClient(config types.IPCConfig) *IPCClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &IPCClient{
		config:      config,
		receiveChan: make(chan types.IPCMessage, config.BufferSize),
		sendChan:    make(chan []byte, config.BufferSize),
		handlers:    make(map[string]func(types.IPCMessage)),
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (c *IPCClient) Connect() error {
	address := fmt.Sprintf("%s:%d", c.config.Address, c.config.Port)

	var err error
	c.conn, err = net.DialTimeout("tcp", address, c.config.Timeout)
	if err != nil {
		return fmt.Errorf("failed to connect to IPC server: %w", err)
	}

	c.decoder = json.NewDecoder(c.conn)
	c.encoder = json.NewEncoder(c.conn)
	c.connected = true

	c.wg.Add(2)
	go c.receiveMessages()
	go c.sendMessages()

	log.Printf("Connected to IPC server at %s", address)
	return nil
}

func (c *IPCClient) Disconnect() {
	if !c.connected {
		return
	}

	// Signal shutdown
	c.cancel()
	c.connected = false

	// Close the connection
	if c.conn != nil {
		_ = c.conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
		c.conn.Close()
	}

	// Safely close channels
	select {
	case <-c.receiveChan: // Already closed
	default:
		close(c.receiveChan)
	}

	select {
	case <-c.sendChan: // Already closed
	default:
		close(c.sendChan)
	}

	// Wait for goroutines to finish with timeout
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Printf("Client disconnected gracefully")
	case <-time.After(3 * time.Second):
		log.Printf("Client disconnect timeout, forcing shutdown")
	}
}

func (c *IPCClient) Send(message types.IPCMessage) error {
	if !c.connected {
		return fmt.Errorf("not connected to server")
	}

	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	select {
	case c.sendChan <- data:
		return nil
	case <-c.ctx.Done():
		return fmt.Errorf("client shutting down")
	case <-time.After(c.config.Timeout):
		return fmt.Errorf("send timeout")
	}
}

func (c *IPCClient) Receive() <-chan types.IPCMessage {
	return c.receiveChan
}

func (c *IPCClient) RegisterHandler(messageType string, handler func(types.IPCMessage)) {
	c.handlersLock.Lock()
	defer c.handlersLock.Unlock()
	c.handlers[messageType] = handler
}

func (c *IPCClient) receiveMessages() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.receiveChan: // Channel closed, time to exit
			return
		default:
			// Set read deadline to detect connection issues
			if err := c.conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
				if !c.connected {
					return
				}
				log.Printf("Set read deadline error: %v", err)
				c.connected = false
				return
			}

			var message types.IPCMessage
			if err := c.decoder.Decode(&message); err != nil {
				if errors.Is(err, io.EOF) {
					log.Printf("Server disconnected gracefully")
				} else if errors.Is(err, net.ErrClosed) {
					log.Printf("Connection closed")
				} else if !c.connected {
					return
				} else {
					log.Printf("Receive error: %v", err)
				}
				c.connected = false
				return
			}

			// Reset deadline after successful read
			_ = c.conn.SetReadDeadline(time.Time{})

			c.routeMessage(message)
		}
	}
}

func (c *IPCClient) sendMessages() {
	defer c.wg.Done()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.sendChan: // Channel closed, time to exit
			return
		case data, ok := <-c.sendChan:
			if !ok {
				return
			}

			if !c.connected {
				return
			}

			// Set write deadline
			if err := c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				log.Printf("Set write deadline error: %v", err)
				c.connected = false
				return
			}

			if _, err := c.conn.Write(data); err != nil {
				if errors.Is(err, net.ErrClosed) {
					log.Printf("Connection closed during send")
				} else {
					log.Printf("Send error: %v", err)
				}
				c.connected = false
				return
			}

			// Reset write deadline
			_ = c.conn.SetWriteDeadline(time.Time{})
		}
	}
}

func (c *IPCClient) routeMessage(message types.IPCMessage) {
	c.handlersLock.RLock()
	handler, exists := c.handlers[message.Type]
	c.handlersLock.RUnlock()

	if exists {
		handler(message)
	} else {
		select {
		case c.receiveChan <- message:
			// Message successfully queued
		case <-c.ctx.Done():
			// Client is shutting down
		case <-time.After(100 * time.Millisecond):
			log.Printf("Receive channel full, dropping message type: %s", message.Type)
		}
	}
}