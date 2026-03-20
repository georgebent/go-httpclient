package gohttp_mock

import (
	"net/http"
	"sync"

	"github.com/georgebent/go-httpclient/core"
)

var (
	MockupServer = mockServer{
		transport: NewTransport(),
	}
)

type mockServer struct {
	enabled     bool
	serverMutex sync.RWMutex
	transport   *MockTransport
}

func StartMockServer() {
	MockupServer.serverMutex.Lock()
	defer MockupServer.serverMutex.Unlock()

	MockupServer.enabled = true
}

func StopMockServer() {
	MockupServer.serverMutex.Lock()
	defer MockupServer.serverMutex.Unlock()

	MockupServer.enabled = false
}

func (m *mockServer) IsMockServerEnabled() bool {
	m.serverMutex.RLock()
	defer m.serverMutex.RUnlock()

	return m.enabled
}

func (m *mockServer) DeleteMocks() {
	m.transport.DeleteMocks()
}

func AddMock(mock Mock) {
	MockupServer.transport.AddMock(mock)
}

func (m *mockServer) GetMockedTransport() http.RoundTripper {
	m.serverMutex.RLock()
	defer m.serverMutex.RUnlock()

	return m.transport
}

func (m *mockServer) GetMockedClient() core.HttpClient {
	m.serverMutex.RLock()
	defer m.serverMutex.RUnlock()

	return &httpClientMock{
		transport: m.transport,
	}
}
