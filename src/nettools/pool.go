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

const defaultICMPReadBufferBytes = 4 * 1024 * 1024

// icmpResponse 是 readLoop 分发给等待者的响应
type icmpResponse struct {
	addr  net.Addr
	final bool // EchoReply
	down  bool // DestinationUnreachable
}

// icmpPool 全局唯一 ICMP 连接池
type icmpPool struct {
	initMu       sync.Mutex
	conn         net.PacketConn
	ipconn       *ipv4.PacketConn
	listenPacket func(network, address string) (net.PacketConn, error)

	mu      sync.RWMutex
	waiters map[uint32]chan icmpResponse

	sendMu sync.Mutex // 保护 SetTTL + WriteTo 原子操作
}

var pool icmpPool

// waiterKey 生成等待者唯一标识
func waiterKey(id, seq int) uint32 {
	return uint32((id&0xFFFF)<<16 | (seq & 0xFFFF))
}

// init 惰性初始化全局 socket
func (p *icmpPool) init() error {
	p.initMu.Lock()
	defer p.initMu.Unlock()
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
		p.waiters = make(map[uint32]chan icmpResponse)
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
func (p *icmpPool) register(key uint32) (chan icmpResponse, bool) {
	ch := make(chan icmpResponse, 1)
	p.mu.Lock()
	if _, exists := p.waiters[key]; exists {
		p.mu.Unlock()
		return nil, false
	}
	p.waiters[key] = ch
	p.mu.Unlock()
	return ch, true
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

func CloseICMPPool() error {
	return pool.close()
}

func (p *icmpPool) close() error {
	p.initMu.Lock()
	defer p.initMu.Unlock()
	p.sendMu.Lock()
	defer p.sendMu.Unlock()
	if p.conn == nil {
		return nil
	}
	err := p.conn.Close()
	p.conn = nil
	p.ipconn = nil
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
	if err := p.init(); err != nil {
		return ICMP{Error: err}
	}

	key := waiterKey(id, seq)
	ch, registered := p.register(key)
	if !registered {
		return ICMP{Error: errors.New("icmp request identifier collision")}
	}
	defer p.unregister(key)

	// SetTTL + WriteTo 必须原子执行
	p.sendMu.Lock()
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

	return waitForICMPResponse(ctx, ch, sendOn, timeout)
}

func waitForICMPResponse(ctx context.Context, ch <-chan icmpResponse, sendOn time.Time, timeout time.Duration) ICMP {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case resp := <-ch:
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
