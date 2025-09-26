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

type Client struct {
	ID        string
	Conn      net.Conn
	Send      chan []byte
	Active    bool
	closeOnce sync.Once
	closed    chan struct{}
}

type IPCServer struct {
	config      types.IPCConfig
	clients     map[string]*Client
	clientsLock sync.RWMutex
	handlers    map[string]func(types.IPCMessage)
	handlersLock sync.RWMutex
	server      net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func NewIPCServer(config types.IPCConfig) *IPCServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &IPCServer{
		config:   config,
		clients:  make(map[string]*Client),
		handlers: make(map[string]func(types.IPCMessage)),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (s *IPCServer) Start() error {
	var err error
	address := net.JoinHostPort(s.config.Address, fmt.Sprintf("%d", s.config.Port))

	s.server, err = net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to start IPC server: %w", err)
	}

	log.Printf("IPC server started on %s", address)

	s.wg.Add(1)
	go s.acceptConnections()

	return nil
}

func (s *IPCServer) Stop() error {
	s.cancel()

	if s.server != nil {
		s.server.Close()
	}

	s.clientsLock.Lock()
	for _, client := range s.clients {
		s.safeCloseClient(client)
	}
	s.clients = make(map[string]*Client)
	s.clientsLock.Unlock()

	s.wg.Wait()
	return nil
}

// safeCloseClient safely closes a client connection and its channels
func (s *IPCServer) safeCloseClient(client *Client) {
	client.closeOnce.Do(func() {
		// Signal that we're closing
		close(client.closed)

		// Close the connection
		if client.Conn != nil {
			client.Conn.Close()
		}

		// Close the send channel if it's not already closed
		select {
		case <-client.Send: // Check if already closed
		default:
			close(client.Send)
		}

		// Mark as inactive
		client.Active = false

		log.Printf("Client %s safely closed", client.ID)
	})
}

func (s *IPCServer) acceptConnections() {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			conn, err := s.server.Accept()
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					log.Printf("Accept error: %v", err)
				}
				continue
			}

			clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
			client := &Client{
				ID:     clientID,
				Conn:   conn,
				Send:   make(chan []byte, s.config.BufferSize),
				Active: true,
				closed: make(chan struct{}),
			}

			s.clientsLock.Lock()
			s.clients[clientID] = client
			s.clientsLock.Unlock()

			s.wg.Add(2)
			go s.handleClient(client)
			go s.sendToClient(client)

			log.Printf("Client connected: %s", clientID)
		}
	}
}

func (s *IPCServer) handleClient(client *Client) {
	defer s.wg.Done()
	defer s.safeCloseClient(client)

	decoder := json.NewDecoder(client.Conn)

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-client.closed:
			return
		default:
			// Set a deadline for reading to detect client disconnection
			if err := client.Conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
				log.Printf("Client %s set read deadline error: %v", client.ID, err)
				return
			}

			var message types.IPCMessage
			if err := decoder.Decode(&message); err != nil {
				if errors.Is(err, io.EOF) {
					log.Printf("Client %s disconnected gracefully", client.ID)
				} else if !errors.Is(err, net.ErrClosed) {
					log.Printf("Client %s decode error: %v", client.ID, err)
				}
				return
			}

			// Reset deadline after successful read
			_ = client.Conn.SetReadDeadline(time.Time{})

			message.Source = client.ID
			s.routeMessage(message)
		}
	}
}

func (s *IPCServer) sendToClient(client *Client) {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-client.closed:
			return
		case data, ok := <-client.Send:
			if !ok {
				return
			}

			// Check if client is still active before sending
			if !client.Active {
				return
			}

			// Set write deadline
			if err := client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				log.Printf("Client %s set write deadline error: %v", client.ID, err)
				return
			}

			_, err := client.Conn.Write(data)
			if err != nil {
				if !errors.Is(err, net.ErrClosed) {
					log.Printf("Send to client %s error: %v", client.ID, err)
				}
				return
			}

			// Reset write deadline
			_ = client.Conn.SetWriteDeadline(time.Time{})
		}
	}
}

func (s *IPCServer) routeMessage(message types.IPCMessage) {
	s.handlersLock.RLock()
	handler, exists := s.handlers[message.Type]
	s.handlersLock.RUnlock()

	if exists {
		handler(message)
	}
}

func (s *IPCServer) Broadcast(message types.IPCMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	s.clientsLock.RLock()
	defer s.clientsLock.RUnlock()

	for _, client := range s.clients {
		if client.Active {
			select {
			case client.Send <- data:
			default:
				log.Printf("Client %s send buffer full", client.ID)
			}
		}
	}

	return nil
}

func (s *IPCServer) SendToClient(clientID string, message types.IPCMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	s.clientsLock.RLock()
	client, exists := s.clients[clientID]
	s.clientsLock.RUnlock()

	if !exists {
		return fmt.Errorf("client not found: %s", clientID)
	}

	// Check if client is still active
	if !client.Active {
		return fmt.Errorf("client not active: %s", clientID)
	}

	select {
	case client.Send <- data:
		return nil
	case <-client.closed:
		return fmt.Errorf("client closed: %s", clientID)
	case <-time.After(5 * time.Second):
		return fmt.Errorf("send timeout for client: %s", clientID)
	}
}

func (s *IPCServer) RegisterHandler(messageType string, handler func(types.IPCMessage)) {
	s.handlersLock.Lock()
	defer s.handlersLock.Unlock()
	s.handlers[messageType] = handler
}