package nettools

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	defaultICMPReadBufferBytes = 4 * 1024 * 1024
	ipv4ProtocolICMP           = 1
)

// icmpResponse 是 readLoop 分发给等待者的响应
type icmpResponse struct {
	addr  net.Addr
	final bool // EchoReply
	down  bool // DestinationUnreachable
}

type icmpWaiter struct {
	responses   chan icmpResponse
	destination net.Addr
}

// icmpPool 全局唯一 ICMP 连接池
type icmpPool struct {
	initMu       sync.Mutex
	conn         net.PacketConn
	ipconn       *ipv4.PacketConn
	listenPacket func(network, address string) (net.PacketConn, error)

	mu      sync.RWMutex
	waiters map[uint32]icmpWaiter

	sendMu sync.Mutex // 保护 SetTTL + WriteTo 原子操作
}

var pool icmpPool

// waiterKey 生成等待者唯一标识
func waiterKey(id, seq int) uint32 {
	return uint32((id&0xFFFF)<<16 | (seq & 0xFFFF))
}

func embeddedEchoWaiterKey(data []byte) (uint32, bool) {
	if len(data) < ipv4.HeaderLen || data[0]>>4 != 4 {
		return 0, false
	}
	headerLength := int(data[0]&0x0f) * 4
	if headerLength < ipv4.HeaderLen || len(data) < headerLength+8 || data[9] != ipv4ProtocolICMP {
		return 0, false
	}
	if data[headerLength] != byte(ipv4.ICMPTypeEcho) || data[headerLength+1] != 0 {
		return 0, false
	}
	id := int(binary.BigEndian.Uint16(data[headerLength+4 : headerLength+6]))
	seq := int(binary.BigEndian.Uint16(data[headerLength+6 : headerLength+8]))
	return waiterKey(id, seq), true
}

// init 惰性初始化全局 socket
func (p *icmpPool) init() error {
	p.initMu.Lock()
	defer p.initMu.Unlock()
	return p.initLocked()
}

// initLocked requires initMu to be held by the caller.
func (p *icmpPool) initLocked() error {
	if p.conn != nil {
		return nil
	}

	listenPacket := p.listenPacket
	if listenPacket == nil {
		listenPacket = net.ListenPacket
	}
	conn, err := listenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return err
	}
	if p.waiters == nil {
		p.waiters = make(map[uint32]icmpWaiter)
	}
	if connWithReadBuffer, ok := conn.(interface{ SetReadBuffer(bytes int) error }); ok {
		if err := connWithReadBuffer.SetReadBuffer(defaultICMPReadBufferBytes); err != nil {
			logrus.Warnf("[icmpPool:init] SetReadBuffer(%d) failed: %v", defaultICMPReadBufferBytes, err)
		}
	}
	p.conn = conn
	p.ipconn = ipv4.NewPacketConn(conn)
	go p.readLoop(conn)
	return nil
}

// register 注册一个等待者，返回接收 channel
func (p *icmpPool) register(key uint32, destination net.Addr) (chan icmpResponse, bool) {
	ch := make(chan icmpResponse, 1)
	p.mu.Lock()
	if _, exists := p.waiters[key]; exists {
		p.mu.Unlock()
		return nil, false
	}
	p.waiters[key] = icmpWaiter{responses: ch, destination: destination}
	p.mu.Unlock()
	return ch, true
}

// unregister 移除等待者
func (p *icmpPool) unregister(key uint32, responses chan icmpResponse) {
	p.mu.Lock()
	// A closed pool may already have a new request using the same identifier.
	if waiter, exists := p.waiters[key]; exists && waiter.responses == responses {
		delete(p.waiters, key)
	}
	p.mu.Unlock()
}

func (p *icmpPool) dispatchFrom(conn net.PacketConn, key uint32, resp icmpResponse) {
	p.initMu.Lock()
	defer p.initMu.Unlock()
	if p.conn == conn {
		p.dispatch(key, resp)
	}
}

// dispatch 将响应分发给对应等待者
func (p *icmpPool) dispatch(key uint32, resp icmpResponse) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	waiter, ok := p.waiters[key]
	if ok {
		if resp.final && !sameIPAddress(resp.addr, waiter.destination) {
			return
		}
		select {
		case waiter.responses <- resp:
		default:
		}
	}
}

// readLoop 持续读取所有 ICMP 报文并按 (ID, Seq) 分发
func (p *icmpPool) readLoop(conn net.PacketConn) {
	buf := make([]byte, 1500)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// 瞬态错误（Windows WSAECONNRESET 等），继续读取
			logrus.Debug("[icmpPool:readLoop] ReadFrom error: ", err)
			time.Sleep(10 * time.Millisecond)
			continue
		}

		// Go 的 IPConn.ReadFrom 已通过 stripIPv4Header 剥离 IP 头，
		// 此处 buf[:n] 即为纯 ICMP 载荷。
		msg, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			continue
		}

		switch msg.Type {
		case ipv4.ICMPTypeEchoReply:
			echo, ok := msg.Body.(*icmp.Echo)
			if !ok || msg.Code != 0 {
				continue
			}
			key := waiterKey(echo.ID, echo.Seq)
			p.dispatchFrom(conn, key, icmpResponse{addr: addr, final: true})

		case ipv4.ICMPTypeTimeExceeded:
			te, ok := msg.Body.(*icmp.TimeExceeded)
			if !ok {
				continue
			}
			key, ok := embeddedEchoWaiterKey(te.Data)
			if !ok {
				continue
			}
			p.dispatchFrom(conn, key, icmpResponse{addr: addr})

		case ipv4.ICMPTypeDestinationUnreachable:
			du, ok := msg.Body.(*icmp.DstUnreach)
			if !ok {
				continue
			}
			key, ok := embeddedEchoWaiterKey(du.Data)
			if !ok {
				continue
			}
			p.dispatchFrom(conn, key, icmpResponse{addr: addr, down: true})
		}
	}
}

func CloseICMPPool() error {
	return pool.close()
}

func (p *icmpPool) close() error {
	p.initMu.Lock()
	defer p.initMu.Unlock()
	p.sendMu.Lock()
	defer p.sendMu.Unlock()
	var err error
	if p.conn != nil {
		err = p.conn.Close()
		p.conn = nil
		p.ipconn = nil
	}
	p.mu.Lock()
	for key, waiter := range p.waiters {
		close(waiter.responses)
		delete(p.waiters, key)
	}
	p.mu.Unlock()
	return err
}

// sendICMP 发送 ICMP 报文并等待响应
func (p *icmpPool) sendICMP(id, seq, ttl int, msg []byte, dest net.Addr, timeout time.Duration) ICMP {
	return p.sendICMPContext(context.Background(), id, seq, ttl, msg, dest, timeout)
}

func (p *icmpPool) sendICMPContext(ctx context.Context, id, seq, ttl int, msg []byte, dest net.Addr, timeout time.Duration) ICMP {
	if err := ctx.Err(); err != nil {
		return ICMP{Error: err}
	}
	// Keep initialization and registration in the same lifecycle as the send.
	p.initMu.Lock()
	if err := p.initLocked(); err != nil {
		p.initMu.Unlock()
		return ICMP{Error: err}
	}

	key := waiterKey(id, seq)
	ch, registered := p.register(key, dest)
	if !registered {
		p.initMu.Unlock()
		return ICMP{Error: errors.New("icmp request identifier collision")}
	}
	defer p.unregister(key, ch)

	// SetTTL + WriteTo 必须原子执行
	p.sendMu.Lock()
	p.initMu.Unlock()
	if p.conn == nil || p.ipconn == nil {
		p.sendMu.Unlock()
		return ICMP{Error: net.ErrClosed}
	}
	if err := ctx.Err(); err != nil {
		p.sendMu.Unlock()
		return ICMP{Error: err}
	}
	err := p.ipconn.SetTTL(ttl)
	if err != nil {
		p.sendMu.Unlock()
		return ICMP{Error: err}
	}
	sendOn := time.Now()
	_, err = p.conn.WriteTo(msg, dest)
	p.sendMu.Unlock()
	if err != nil {
		return ICMP{Error: err}
	}

	return waitForICMPResponse(ctx, ch, sendOn, timeout, dest)
}

func waitForICMPResponse(ctx context.Context, ch <-chan icmpResponse, sendOn time.Time, timeout time.Duration, destination net.Addr) ICMP {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case resp, ok := <-ch:
			if !ok {
				return ICMP{Error: net.ErrClosed}
			}
			if resp.final && !sameIPAddress(resp.addr, destination) {
				continue
			}
			return ICMP{
				Addr:  resp.addr,
				RTT:   time.Since(sendOn),
				Final: resp.final,
				Down:  resp.down,
			}
		case <-timer.C:
			return ICMP{Timeout: true}
		case <-ctx.Done():
			return ICMP{Error: ctx.Err()}
		}
	}
}

func sameIPAddress(left, right net.Addr) bool {
	leftIP, leftOK := left.(*net.IPAddr)
	rightIP, rightOK := right.(*net.IPAddr)
	return leftOK && rightOK && leftIP != nil && rightIP != nil && leftIP.IP.Equal(rightIP.IP)
}
