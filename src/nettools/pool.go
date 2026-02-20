package nettools

import (
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// icmpResponse 是 readLoop 分发给等待者的响应
type icmpResponse struct {
	addr  net.Addr
	rtt   time.Duration
	final bool // EchoReply
	down  bool // DestinationUnreachable
}

// icmpPool 全局唯一 ICMP 连接池
type icmpPool struct {
	once   sync.Once
	conn   net.PacketConn
	ipconn *ipv4.PacketConn
	initErr error

	mu      sync.RWMutex
	waiters map[uint32]chan icmpResponse

	sendMu sync.Mutex // 保护 SetTTL + WriteTo 原子操作
}

var pool icmpPool

// waiterKey 生成等待者唯一标识
func waiterKey(id, seq int) uint32 {
	return uint32((id & 0xFFFF) << 16 | (seq & 0xFFFF))
}

// init 惰性初始化全局 socket
func (p *icmpPool) init() error {
	p.once.Do(func() {
		p.waiters = make(map[uint32]chan icmpResponse)
		p.conn, p.initErr = net.ListenPacket("ip4:icmp", "0.0.0.0")
		if p.initErr != nil {
			return
		}
		p.ipconn = ipv4.NewPacketConn(p.conn)
		go p.readLoop()
	})
	return p.initErr
}

// register 注册一个等待者，返回接收 channel
func (p *icmpPool) register(key uint32) chan icmpResponse {
	ch := make(chan icmpResponse, 1)
	p.mu.Lock()
	p.waiters[key] = ch
	p.mu.Unlock()
	return ch
}

// unregister 移除等待者
func (p *icmpPool) unregister(key uint32) {
	p.mu.Lock()
	delete(p.waiters, key)
	p.mu.Unlock()
}

// dispatch 将响应分发给对应等待者
func (p *icmpPool) dispatch(key uint32, resp icmpResponse) {
	p.mu.RLock()
	ch, ok := p.waiters[key]
	p.mu.RUnlock()
	if ok {
		select {
		case ch <- resp:
		default:
		}
	}
}

// readLoop 持续读取所有 ICMP 报文并按 (ID, Seq) 分发
func (p *icmpPool) readLoop() {
	buf := make([]byte, 1500)
	for {
		n, addr, err := p.conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			// 瞬态错误（Windows WSAECONNRESET 等），继续读取
			logrus.Debug("[icmpPool:readLoop] ReadFrom error: ", err)
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
			if !ok {
				continue
			}
			key := waiterKey(echo.ID, echo.Seq)
			p.dispatch(key, icmpResponse{addr: addr, final: true})

		case ipv4.ICMPTypeTimeExceeded:
			te, ok := msg.Body.(*icmp.TimeExceeded)
			if !ok || len(te.Data) < 28 {
				continue
			}
			id := int(binary.BigEndian.Uint16(te.Data[24:26]))
			seq := int(binary.BigEndian.Uint16(te.Data[26:28]))
			key := waiterKey(id, seq)
			p.dispatch(key, icmpResponse{addr: addr})

		case ipv4.ICMPTypeDestinationUnreachable:
			du, ok := msg.Body.(*icmp.DstUnreach)
			if !ok || len(du.Data) < 28 {
				continue
			}
			id := int(binary.BigEndian.Uint16(du.Data[24:26]))
			seq := int(binary.BigEndian.Uint16(du.Data[26:28]))
			key := waiterKey(id, seq)
			p.dispatch(key, icmpResponse{addr: addr, down: true})
		}
	}
}

// sendICMP 发送 ICMP 报文并等待响应
func (p *icmpPool) sendICMP(id, seq, ttl int, msg []byte, dest net.Addr, timeout time.Duration) ICMP {
	if err := p.init(); err != nil {
		return ICMP{Error: err}
	}

	key := waiterKey(id, seq)
	ch := p.register(key)
	defer p.unregister(key)

	// SetTTL + WriteTo 必须原子执行
	p.sendMu.Lock()
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

	select {
	case resp := <-ch:
		return ICMP{
			Addr:  resp.addr,
			RTT:   time.Since(sendOn),
			Final: resp.final,
			Down:  resp.down,
		}
	case <-time.After(timeout):
		return ICMP{Timeout: true}
	}
}
