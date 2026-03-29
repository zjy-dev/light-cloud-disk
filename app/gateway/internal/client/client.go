package client

import (
	"context"
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	consulapi "github.com/hashicorp/consul/api"
	googlegrpc "google.golang.org/grpc"

	filev1 "github.com/J-Y-Zhang/light-cloud-disk/api/file/v1"
	userv1 "github.com/J-Y-Zhang/light-cloud-disk/api/user/v1"
)

type ServiceClients struct {
	User userv1.UserServiceClient
	File filev1.FileServiceClient
	// fileRouter routes upload-related requests by MD5 consistent hash
	// when multiple file-service instances are discovered. Falls back to
	// the default File client when only one instance exists.
	fileRouter *hashRouter
}

// FileClientByKey returns a FileServiceClient pinned to the file-service
// instance selected by consistent-hashing on the given key (typically file MD5).
// For single-instance deployments this returns the default File client.
func (sc *ServiceClients) FileClientByKey(key string) filev1.FileServiceClient {
	if sc.fileRouter == nil {
		return sc.File
	}
	client := sc.fileRouter.pick(key)
	if client == nil {
		return sc.File
	}
	return client
}

// PickNFileHTTPAddrs returns up to n distinct HTTP addresses of file-service
// instances, selected by consistent-hashing on key, for per-chunk upload routing.
func (sc *ServiceClients) PickNFileHTTPAddrs(key string, n int) []string {
	if sc.fileRouter == nil {
		return nil
	}
	return sc.fileRouter.pickN(key, n)
}

func NewServiceClients() *ServiceClients {
	r := newRegistry()

	userConn, err := dialWithRetry(r, "discovery:///user-service")
	if err != nil {
		panic(err)
	}

	fileConn, err := dialWithRetry(r, "discovery:///file-service")
	if err != nil {
		panic(err)
	}

	sc := &ServiceClients{
		User: userv1.NewUserServiceClient(userConn),
		File: filev1.NewFileServiceClient(fileConn),
	}

	// Initialize hash router for multi-instance file-service routing.
	consulAddr := os.Getenv("CONSUL_ADDR")
	if consulAddr == "" {
		consulAddr = "127.0.0.1:8500"
	}
	sc.fileRouter = newHashRouter(consulAddr, "file-service")

	return sc
}

// ---- Consistent hash router ----

const virtualNodes = 150 // virtual nodes per real node for balanced distribution

// hashRouterEntry holds a gRPC connection and its client.
type hashRouterEntry struct {
	conn   *googlegrpc.ClientConn
	client filev1.FileServiceClient
}

// hashRouter provides consistent hash-based routing to file-service instances.
// It periodically watches Consul for instance changes and maintains per-address
// gRPC connections. Unhealthy instances are temporarily marked and skipped for
// a cooldown period before being retried.
type hashRouter struct {
	serviceName string
	consulAddr  string

	mu        sync.RWMutex
	ring      []uint32                    // sorted virtual-node hashes
	ringMap   map[uint32]string           // hash → address
	clients   map[string]*hashRouterEntry // address → gRPC entry
	httpAddrs map[string]string           // gRPC addr → HTTP addr (host:httpPort)

	// unhealthy tracks temporarily marked-down instances and their retry time.
	unhealthy map[string]time.Time
}

const unhealthyCooldown = 15 * time.Second

func newHashRouter(consulAddr, serviceName string) *hashRouter {
	hr := &hashRouter{
		serviceName: serviceName,
		consulAddr:  consulAddr,
		ringMap:     make(map[uint32]string),
		clients:     make(map[string]*hashRouterEntry),
		httpAddrs:   make(map[string]string),
		unhealthy:   make(map[string]time.Time),
	}
	// Initial population
	hr.refresh()
	// Background refresh every 15s
	go hr.watchLoop()
	return hr
}

func (hr *hashRouter) watchLoop() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		hr.refresh()
	}
}

func (hr *hashRouter) refresh() {
	consulCli, err := consulapi.NewClient(&consulapi.Config{Address: hr.consulAddr})
	if err != nil {
		log.Warnf("hashRouter: consul client error: %v", err)
		return
	}
	entries, _, err := consulCli.Health().Service(hr.serviceName, "", true, nil)
	if err != nil {
		log.Warnf("hashRouter: consul query error: %v", err)
		return
	}

	type instanceInfo struct {
		grpcAddr string
		httpAddr string // host:httpPort, empty if no http_port meta
	}
	instances := make([]instanceInfo, 0, len(entries))
	addrs := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		grpcAddr := fmt.Sprintf("%s:%d", e.Service.Address, e.Service.Port)
		addrs[grpcAddr] = struct{}{}
		httpAddr := ""
		if hp, ok := e.Service.Meta["http_port"]; ok && hp != "" {
			httpAddr = fmt.Sprintf("%s:%s", e.Service.Address, hp)
		}
		instances = append(instances, instanceInfo{grpcAddr: grpcAddr, httpAddr: httpAddr})
	}

	hr.mu.Lock()
	defer hr.mu.Unlock()

	// Detect changes for logging
	prevCount := len(hr.clients)
	var added, removed []string
	for addr := range addrs {
		if _, ok := hr.clients[addr]; !ok {
			added = append(added, addr)
		}
	}
	for addr := range hr.clients {
		if _, ok := addrs[addr]; !ok {
			removed = append(removed, addr)
		}
	}

	// Rebuild ring
	hr.ring = hr.ring[:0]
	hr.ringMap = make(map[uint32]string, len(addrs)*virtualNodes)
	for addr := range addrs {
		for i := range virtualNodes {
			h := fnvHash(fmt.Sprintf("%s#%d", addr, i))
			hr.ring = append(hr.ring, h)
			hr.ringMap[h] = addr
		}
	}
	sort.Slice(hr.ring, func(i, j int) bool { return hr.ring[i] < hr.ring[j] })

	// Create connections for new addresses, remove stale ones
	for addr := range addrs {
		if _, ok := hr.clients[addr]; ok {
			continue
		}
		conn, err := googlegrpc.NewClient(addr, googlegrpc.WithInsecure())
		if err != nil {
			log.Warnf("hashRouter: dial %s error: %v", addr, err)
			continue
		}
		hr.clients[addr] = &hashRouterEntry{
			conn:   conn,
			client: filev1.NewFileServiceClient(conn),
		}
	}
	for addr, entry := range hr.clients {
		if _, ok := addrs[addr]; !ok {
			_ = entry.conn.Close()
			delete(hr.clients, addr)
		}
	}

	// Rebuild HTTP address map
	hr.httpAddrs = make(map[string]string, len(instances))
	for _, inst := range instances {
		if inst.httpAddr != "" {
			hr.httpAddrs[inst.grpcAddr] = inst.httpAddr
		}
	}

	// Purge expired unhealthy entries; remove entries for instances no longer in Consul
	now := time.Now()
	for addr := range hr.unhealthy {
		if _, ok := addrs[addr]; !ok {
			delete(hr.unhealthy, addr)
		} else if now.After(hr.unhealthy[addr]) {
			delete(hr.unhealthy, addr)
		}
	}

	// Log ring rebalance events
	if len(added) > 0 || len(removed) > 0 {
		log.Infof("hashRouter: ring rebalanced — instances %d→%d, added=%v, removed=%v",
			prevCount, len(hr.clients), added, removed)
	}
}

func (hr *hashRouter) pick(key string) filev1.FileServiceClient {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.ring) == 0 {
		return nil
	}

	h := fnvHash(key)
	idx := sort.Search(len(hr.ring), func(i int) bool { return hr.ring[i] >= h })
	if idx >= len(hr.ring) {
		idx = 0
	}

	now := time.Now()
	total := len(hr.ring)
	seen := make(map[string]struct{})
	for i := range total {
		pos := (idx + i) % total
		addr := hr.ringMap[hr.ring[pos]]
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		if t, ok := hr.unhealthy[addr]; ok && now.Before(t) {
			continue // skip unhealthy until cooldown expires
		}
		if entry, ok := hr.clients[addr]; ok {
			return entry.client
		}
	}

	// fallback: try first available (even unhealthy, better than nil)
	addr := hr.ringMap[hr.ring[idx]]
	if entry, ok := hr.clients[addr]; ok {
		return entry.client
	}
	return nil
}

// pickN walks the hash ring from the key's position and returns up to n distinct
// physical node HTTP addresses, skipping temporarily unhealthy instances.
func (hr *hashRouter) pickN(key string, n int) []string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.ring) == 0 {
		return nil
	}

	h := fnvHash(key)
	idx := sort.Search(len(hr.ring), func(i int) bool { return hr.ring[i] >= h })
	if idx >= len(hr.ring) {
		idx = 0
	}

	now := time.Now()
	seen := make(map[string]struct{})
	var result []string
	total := len(hr.ring)
	for i := range total {
		pos := (idx + i) % total
		grpcAddr := hr.ringMap[hr.ring[pos]]
		if _, ok := seen[grpcAddr]; ok {
			continue
		}
		seen[grpcAddr] = struct{}{}
		if t, ok := hr.unhealthy[grpcAddr]; ok && now.Before(t) {
			continue
		}
		if httpAddr, ok := hr.httpAddrs[grpcAddr]; ok {
			result = append(result, httpAddr)
		}
		if len(result) >= n {
			break
		}
	}
	return result
}

// GetHTTPAddrs returns all known HTTP addresses of healthy file-service instances.
func (hr *hashRouter) GetHTTPAddrs() []string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	addrs := make([]string, 0, len(hr.httpAddrs))
	for _, addr := range hr.httpAddrs {
		addrs = append(addrs, addr)
	}
	return addrs
}

// MarkUnhealthy temporarily marks a file-service HTTP address as unhealthy
// for the cooldown period. The instance is skipped in pick/pickN until
// the cooldown expires, then it's retried.
func (hr *hashRouter) MarkUnhealthy(httpAddr string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()
	// Find gRPC addr by HTTP addr (reverse lookup)
	for grpcAddr, ha := range hr.httpAddrs {
		if ha == httpAddr {
			hr.unhealthy[grpcAddr] = time.Now().Add(unhealthyCooldown)
			log.Warnf("hashRouter: marked %s (http=%s) unhealthy for %v", grpcAddr, httpAddr, unhealthyCooldown)
			return
		}
	}
}

// MarkFileInstanceUnhealthy exposes MarkUnhealthy on ServiceClients.
func (sc *ServiceClients) MarkFileInstanceUnhealthy(httpAddr string) {
	if sc.fileRouter != nil {
		sc.fileRouter.MarkUnhealthy(httpAddr)
	}
}

func fnvHash(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}

// dialWithRetry dials a gRPC service via Consul discovery, retrying up to 10 times
// with a 10-second discovery timeout per attempt.
func dialWithRetry(r *consul.Registry, endpoint string) (*googlegrpc.ClientConn, error) {
	var conn *googlegrpc.ClientConn
	var err error
	for i := 0; i < 10; i++ {
		conn, err = grpc.DialInsecure(
			context.Background(),
			grpc.WithEndpoint(endpoint),
			grpc.WithDiscovery(r),
			grpc.WithTimeout(10*time.Second),
		)
		if err == nil {
			return conn, nil
		}
		time.Sleep(3 * time.Second)
	}
	return nil, err
}

func newRegistry() *consul.Registry {
	addr := os.Getenv("CONSUL_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8500"
	}

	consulCli, err := consulapi.NewClient(&consulapi.Config{
		Address: addr,
	})
	if err != nil {
		panic(err)
	}
	return consul.New(consulCli)
}
