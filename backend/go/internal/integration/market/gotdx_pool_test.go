package market

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/bensema/gotdx"
	"github.com/stretchr/testify/require"
)

// newTestPool 构造一个不触网的连接池：newClient 返回未连接的 gotdx.Client
//（Disconnect 在 conn 为 nil 时安全），仅用于验证池的复用 / 配额逻辑。
func newTestPool(maxConn int, dials *atomic.Int32) *GotdxPool {
	p := NewGotdxPool(maxConn)
	p.newClient = func() (*gotdx.Client, error) {
		dials.Add(1)
		return gotdx.New(), nil
	}
	return p
}

func TestPoolReusesHealthyConn(t *testing.T) {
	var dials atomic.Int32
	p := newTestPool(2, &dials)

	for i := 0; i < 5; i++ {
		err := p.WithClient(context.Background(), func(_ *gotdx.Client) error { return nil })
		require.NoError(t, err)
	}
	// 健康连接复用，全程只拨号一次。
	require.Equal(t, int32(1), dials.Load())
}

func TestPoolDiscardsUnhealthyConn(t *testing.T) {
	var dials atomic.Int32
	p := newTestPool(2, &dials)

	// fn 始终出错 → 连接每次被丢弃并重建；单次 WithClient 最多重试 3 次。
	err := p.WithClient(context.Background(), func(_ *gotdx.Client) error {
		return errors.New("boom")
	})
	require.Error(t, err)
	require.Equal(t, int32(3), dials.Load())

	// 坏连接已全部归还配额，新的健康调用仍可正常借到连接。
	err = p.WithClient(context.Background(), func(_ *gotdx.Client) error { return nil })
	require.NoError(t, err)
}

func TestPoolRecoversFromPanic(t *testing.T) {
	var dials atomic.Int32
	p := newTestPool(1, &dials)

	err := p.WithClient(context.Background(), func(_ *gotdx.Client) error {
		panic("kaboom")
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "panic")

	// panic 连接被丢弃且配额归还，后续调用不受影响。
	err = p.WithClient(context.Background(), func(_ *gotdx.Client) error { return nil })
	require.NoError(t, err)
}

func TestPoolAcquireRespectsContext(t *testing.T) {
	var dials atomic.Int32
	p := newTestPool(1, &dials)

	// 借出唯一连接且不归还，占满配额。
	c, err := p.acquire(context.Background())
	require.NoError(t, err)
	require.NotNil(t, c)

	// 配额耗尽时，已取消的 ctx 应让 Acquire 立即返回错误而非永久阻塞。
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = p.acquire(ctx)
	require.ErrorIs(t, err, context.Canceled)
}
