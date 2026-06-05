//go:build gotdx

package market

import (
	"context"
	"fmt"
	"sync"

	"github.com/bensema/gotdx"
)

// GotdxPool 是 gotdx 客户端连接池：复用已建立的通达信 TCP 连接，
// 避免每次请求都 Connect/Disconnect（单次握手实测约 1-4s）。
//
// 设计要点：
//   - tokens 为「总连接配额」（cap=maxConn 且初始填满），只有持有 token 才能新建连接，
//     借此把同时存在的连接数严格限制在 maxConn 以内；
//   - idle 缓存空闲且健康的连接，复用期间该连接继续持有它创建时占用的 token；
//   - 连接发生错误 / panic 即视为不可复用，Disconnect 丢弃并归还 token，由后续请求重建
//     （通达信节点不稳定，坏连接续用会导致读写错位，丢弃最稳妥）；
//   - Acquire 支持 ctx 取消 / 超时，避免连接全部借出时无限阻塞。
//
// 任意时刻：idle 中连接数 + 已借出连接数 = 已创建连接数 = maxConn - len(tokens) ≤ maxConn。
// 归还时该连接处于「借出」态，故 idle 至多有 maxConn-1 个，放回不会阻塞。
type GotdxPool struct {
	newClient func() (*gotdx.Client, error)
	idle      chan *gotdx.Client
	tokens    chan struct{}
	mu        sync.Mutex
	closed    bool
}

// NewGotdxPool 创建容量为 maxConn 的 gotdx 连接池（懒建连：首次 Acquire 时才实际拨号）。
func NewGotdxPool(maxConn int) *GotdxPool {
	if maxConn <= 0 {
		maxConn = 10
	}
	p := &GotdxPool{
		idle:   make(chan *gotdx.Client, maxConn),
		tokens: make(chan struct{}, maxConn),
	}
	for i := 0; i < maxConn; i++ {
		p.tokens <- struct{}{}
	}
	p.newClient = defaultGotdxDial
	return p
}

func defaultGotdxDial() (*gotdx.Client, error) {
	c := gotdx.New()
	if _, err := c.Connect(); err != nil {
		return nil, err
	}
	return c, nil
}

// WithClient 从池中借一个连接执行 fn，结束后按健康状况归还或丢弃。
// fn 出错 / panic 会触发最多 maxAttempts 次重试（每次取/建新连接）。
func (p *GotdxPool) WithClient(ctx context.Context, fn func(*gotdx.Client) error) error {
	const maxAttempts = 3
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		c, err := p.acquire(ctx)
		if err != nil {
			return err
		}
		healthy, callErr := safeCall(c, fn)
		p.release(c, healthy)
		if callErr == nil {
			return nil
		}
		lastErr = callErr
	}
	return lastErr
}

// acquire 优先复用空闲连接；无空闲时消费一个配额 token 新建连接；
// 配额耗尽（连接全部借出）时阻塞等待空闲归还或 ctx 取消。
func (p *GotdxPool) acquire(ctx context.Context) (*gotdx.Client, error) {
	select {
	case c := <-p.idle:
		return c, nil
	default:
	}
	select {
	case c := <-p.idle:
		return c, nil
	case <-p.tokens:
		c, err := p.newClient()
		if err != nil {
			// 新建失败，归还配额，避免连接数被永久占用。
			p.tokens <- struct{}{}
			return nil, err
		}
		return c, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *GotdxPool) release(c *gotdx.Client, healthy bool) {
	if c == nil {
		return
	}
	p.mu.Lock()
	closed := p.closed
	p.mu.Unlock()

	if healthy && !closed {
		select {
		case p.idle <- c:
			return
		default:
			// 理论上不会发生（idle 容量足够），兜底丢弃。
		}
	}
	_ = c.Disconnect()
	p.tokens <- struct{}{}
}

// safeCall 执行 fn 并捕获 panic；返回连接是否健康（可复用）与错误。
func safeCall(c *gotdx.Client, fn func(*gotdx.Client) error) (healthy bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			healthy = false
			err = fmt.Errorf("gotdx provider panic: %v", r)
		}
	}()
	err = fn(c)
	return err == nil, err
}

// Close 关闭连接池：标记关闭并断开所有空闲连接（借出中的连接归还时会自行 Disconnect）。
func (p *GotdxPool) Close() {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	p.closed = true
	p.mu.Unlock()

	for {
		select {
		case c := <-p.idle:
			_ = c.Disconnect()
		default:
			return
		}
	}
}
