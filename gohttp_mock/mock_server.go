package gohttp_mock

import (
	"crypto/md5"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/georgebent/go-httpclient/core"
)

var (
	MockupServer = mockServer{
		mocks:      make(map[string]*Mock),
		httpClient: &httpClientMock{},
	}
)

type mockServer struct {
	enabled     bool
	serverMutex sync.RWMutex
	mocks       map[string]*Mock
	httpClient  core.HttpClient
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
	m.serverMutex.Lock()
	defer m.serverMutex.Unlock()

	m.mocks = make(map[string]*Mock)
}

func AddMock(mock Mock) {
	MockupServer.serverMutex.Lock()
	defer MockupServer.serverMutex.Unlock()

	key := MockupServer.getMockKey(mock.Method, mock.Url, mock.RequestBody)

	MockupServer.mocks[key] = &mock
}

func (m *mockServer) getMock(method string, url string, body string) *Mock {
	m.serverMutex.RLock()
	defer m.serverMutex.RUnlock()

	return m.mocks[m.getMockKey(method, url, body)]
}

func (m *mockServer) getMockKey(method string, url string, body string) string {
	hasher := md5.New()
	hasher.Write([]byte(method + url + m.cleanBody(body)))

	return hex.EncodeToString(hasher.Sum(nil))
}

func (m *mockServer) cleanBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}

	body = strings.ReplaceAll(body, "\t", "")
	body = strings.ReplaceAll(body, "\n", "")

	return body
}

func (m *mockServer) GetMockedClient() core.HttpClient {
	m.serverMutex.RLock()
	defer m.serverMutex.RUnlock()

	return m.httpClient
}
