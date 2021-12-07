package pool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type shutdown struct{}

func (c *shutdown) Shutdown() error {
	return nil
}

type connection struct {
	c    IShutdown
	pool Pool
}

func (c *connection) HandleQuick() {
	// time.Sleep(1 * time.Millisecond)
}

func (c *connection) HandleNormal() {
	time.Sleep(20 * time.Millisecond)
}

func (c *connection) HandleSlow() {
	time.Sleep(500 * time.Millisecond)
}

func (c *connection) Shutdown() error {
	return c.pool.Put(context.Background(), c.c, false)
}

func TestListGetPut(t *testing.T) {
	pool := NewList(SetActive(1), SetIdle(1), SetIdleTimeout(90*time.Second), SetWait(false, 10*time.Millisecond))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		return &shutdown{}, nil
	})

	// test Get Put
	conn, err := pool.Get(context.TODO())
	assert.Nil(t, err)
	c1 := connection{pool: pool, c: conn}
	c1.HandleNormal()
	_ = c1.Shutdown()
}

func TestListPut(t *testing.T) {
	id := 0
	type connID struct {
		IShutdown
		id int
	}
	pool := NewList(SetActive(1), SetIdle(1), SetIdleTimeout(1*time.Second))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		id = id + 1
		return &connID{
			IShutdown: &shutdown{},
			id:        id,
		}, nil
	})

	// test Put(ctx, conn, true)
	conn, err := pool.Get(context.TODO())
	assert.Nil(t, err)
	conn1 := conn.(*connID)
	// Put(ctx, conn, true) drop the connection.
	_ = pool.Put(context.TODO(), conn, true)
	conn, err = pool.Get(context.TODO())

	assert.Nil(t, err)
	conn2 := conn.(*connID)
	assert.NotEqual(t, conn1.id, conn2.id)
}

func TestListIdleTimeout(t *testing.T) {
	id := 0
	type connID struct {
		IShutdown
		id int
	}
	pool := NewList(SetActive(1), SetIdle(1), SetIdleTimeout(1*time.Millisecond))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		id = id + 1
		return &connID{id: id, IShutdown: &shutdown{}}, nil
	})
	// test Put(ctx, conn, true)
	conn, err := pool.Get(context.TODO())

	assert.Nil(t, err)
	conn1 := conn.(*connID)
	// Put(ctx, conn, true) drop the connection.
	_ = pool.Put(context.TODO(), conn, false)
	time.Sleep(5 * time.Millisecond)
	// idletimeout and get new conn
	conn, err = pool.Get(context.TODO())

	assert.Nil(t, err)
	conn2 := conn.(*connID)

	assert.NotEqual(t, conn1.id, conn2.id)
}

func TestListContextTimeout(t *testing.T) {
	// new pool
	pool := NewList(SetActive(1), SetIdle(1), SetIdleTimeout(90*time.Second), SetWait(false, 10*time.Millisecond))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		return &shutdown{}, nil
	})
	// test context timeout
	ctx, cancel := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	defer cancel()
	conn, err := pool.Get(ctx)

	assert.Nil(t, err)
	_, err = pool.Get(ctx)
	// context timeout error
	assert.NotNil(t, err)
	_ = pool.Put(context.TODO(), conn, false)
	_, err = pool.Get(ctx)
	assert.Nil(t, err)
}

func TestListPoolExhausted(t *testing.T) {
	// test pool exhausted

	pool := NewList(SetActive(1), SetIdle(1), SetIdleTimeout(90*time.Second))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		return &shutdown{}, nil
	})

	ctx, cancel := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	defer cancel()
	conn, err := pool.Get(context.TODO())
	assert.Nil(t, err)
	_, err = pool.Get(ctx)
	// config active == 1, so no available conns make connection exhausted.
	assert.NotNil(t, err)
	_ = pool.Put(context.TODO(), conn, false)
	_, err = pool.Get(ctx)
	assert.Nil(t, err)
}

func TestListStaleClean(t *testing.T) {
	id := 0
	type connID struct {
		IShutdown
		id int
	}
	pool := NewList(SetActive(1), SetIdle(1), SetIdleTimeout(1*time.Second))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		id = id + 1
		return &connID{id: id, IShutdown: &shutdown{}}, nil
	})
	conn, err := pool.Get(context.TODO())
	assert.Nil(t, err)
	conn1 := conn.(*connID)
	_ = pool.Put(context.TODO(), conn, false)
	conn, err = pool.Get(context.TODO())
	assert.Nil(t, err)
	conn2 := conn.(*connID)
	assert.Equal(t, conn1.id, conn2.id)
	_ = pool.Put(context.TODO(), conn, false)
	// sleep more than idleTimeout
	time.Sleep(2 * time.Second)
	conn, err = pool.Get(context.TODO())
	assert.Nil(t, err)
	conn3 := conn.(*connID)
	assert.NotEqual(t, conn1.id, conn3.id)
}

func BenchmarkList(b *testing.B) {
	pool := NewList(SetActive(30), SetIdle(30), SetIdleTimeout(90*time.Second), SetWait(false, 10*time.Millisecond))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		return &shutdown{}, nil
	})
	for i := 0; i < b.N; i++ {
		conn, err := pool.Get(context.TODO())
		if err != nil {
			b.Error(err)
			continue
		}
		c1 := connection{pool: pool, c: conn}
		c1.HandleQuick()
		_ = pool.Put(context.TODO(), conn, false)
	}
}

func BenchmarkList1(b *testing.B) {
	pool := NewList(SetActive(30), SetIdle(30), SetIdleTimeout(90*time.Second), SetWait(false, 10*time.Millisecond))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		return &shutdown{}, nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get(context.TODO())
			if err != nil {
				b.Error(err)
				continue
			}
			c1 := connection{pool: pool, c: conn}
			c1.HandleQuick()
			_ = c1.Shutdown()
		}
	})
}

func BenchmarkList2(b *testing.B) {
	pool := NewList(SetActive(30), SetIdle(30), SetIdleTimeout(90*time.Second), SetWait(false, 10*time.Millisecond))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		return &shutdown{}, nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get(context.TODO())
			if err != nil {
				b.Error(err)
				continue
			}
			c1 := connection{pool: pool, c: conn}
			c1.HandleNormal()
			_ = c1.Shutdown()
		}
	})
}

func BenchmarkPool3(b *testing.B) {
	pool := NewList(SetActive(30), SetIdle(30), SetIdleTimeout(90*time.Second), SetWait(false, 10*time.Millisecond))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		return &shutdown{}, nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get(context.TODO())
			if err != nil {
				b.Error(err)
				continue
			}
			c1 := connection{pool: pool, c: conn}
			c1.HandleSlow()
			_ = c1.Shutdown()
		}
	})
}

func BenchmarkList4(b *testing.B) {
	pool := NewList(SetActive(30), SetIdle(30), SetIdleTimeout(90*time.Second))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		return &shutdown{}, nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get(context.TODO())
			if err != nil {
				b.Error(err)
				continue
			}
			c1 := connection{pool: pool, c: conn}
			c1.HandleSlow()
			_ = c1.Shutdown()
		}
	})
}

func BenchmarkList5(b *testing.B) {
	pool := NewList(SetActive(30), SetIdle(30), SetIdleTimeout(90*time.Second))
	pool.New(func(ctx context.Context) (IShutdown, error) {
		return &shutdown{}, nil
	})

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.Get(context.TODO())
			if err != nil {
				b.Error(err)
				continue
			}
			c1 := connection{pool: pool, c: conn}
			c1.HandleSlow()
			_ = c1.Shutdown()
		}
	})
}
