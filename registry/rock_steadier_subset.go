package registry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync/atomic"

	"github.com/songzhibin97/gkit/options"
)

// https://dl.acm.org/doi/10.1145/3570937

const Lot = 10

// MagicNumberGeneration 要保证同一个服务种子一致
type MagicNumberGeneration func() int64

var ErrorHasBeenClosed = errors.New("has been closed")

//func corput(n int, base int) float64 {
//	var q float64
//	bk := float64(1) / float64(base)
//	for n > 0 {
//		q += float64(n%base) * bk
//		bk /= float64(base)
//		n /= base
//	}
//	return q
//}

type config struct {
	buf int
}

func SetBufSize(size int) options.Option {
	return func(o interface{}) {
		o.(*config).buf = size
	}
}

type command struct {
	ids  []int
	code int
}

type RockSteadierSubset struct {
	// The number of servers in the cluster.
	clients        []int
	matrixServices atomic.Value // [][]*int
	appendIndex    int
	//serLock        sync.RWMutex
	hasClient  map[int]int    // client: index 只读模式
	hasService map[int][2]int // service: (x,y) 快速置为nil
	col        int

	command chan command
	config  config
	ctx     context.Context
	cancel  context.CancelFunc
	close   int32
}

func (r *RockSteadierSubset) Close() {
	if !atomic.CompareAndSwapInt32(&r.close, 0, 1) {
		return
	}
	r.cancel()
	close(r.command)
}

func (r *RockSteadierSubset) sentinel() {
	go func() {
		for {
			select {
			case c, ok := <-r.command:
				if !ok {
					return
				}
				switch c.code {
				case 1:
					r.addService(c.ids)
				case 2:
					r.removeService(c.ids)
				}

			case <-r.ctx.Done():
				return
			}
		}
	}()
}

func (r *RockSteadierSubset) matrix() string {
	b := strings.Builder{}
	b.WriteString("\t\t")
	for i := 0; i < r.col; i++ {
		b.WriteString(fmt.Sprintf("lot:%d\t", i))
	}
	b.WriteString(fmt.Sprintln())
	matrix := r.matrixServices.Load().([][]*int)
	for idx, client := range r.clients {
		b.WriteString(fmt.Sprintf("client:%d\t", client))
		for _, ss := range matrix[idx] {
			if ss == nil {
				b.WriteString("N\t")
				continue
			} else {
				b.WriteString(fmt.Sprintf("%d \t", *ss))
			}
		}
		b.WriteString(fmt.Sprintln())
	}
	return b.String()
}

func toPoint(i int) *int {
	return &i
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func NewRockSteadierSubset(ctx context.Context, clients, services []int, magicNumberGeneration MagicNumberGeneration, options ...options.Option) *RockSteadierSubset {
	cfg := config{buf: 10}
	for _, option := range options {
		option(&cfg)
	}

	pad := len(clients)
	matrix := make([][]*int, pad)
	col := 0
	for i := 0; i < len(services); i++ {
		matrix[i%pad] = append(matrix[i%pad], toPoint(services[i]))
		col = max(col, len(matrix[i%pad]))
	}
	// padding
	ls := len(services)
	for ; ls%pad != 0; ls++ {
		matrix[ls%pad] = append(matrix[ls%pad], nil)
	}
	shuffle(magicNumberGeneration(), clients, matrix)
	hasClient := make(map[int]int)
	hasService := make(map[int][2]int)
	for idx, client := range clients {
		hasClient[client] = idx
	}
	for x, ss := range matrix {
		for y, v := range ss {
			if v == nil {
				continue
			}
			hasService[*v] = [2]int{x, y}
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	r := &RockSteadierSubset{
		clients:    clients,
		hasClient:  hasClient,
		hasService: hasService,
		col:        col,

		ctx:     ctx,
		cancel:  cancel,
		config:  cfg,
		command: make(chan command, cfg.buf),
	}
	r.matrixServices.Store(matrix)
	return r
}

func shuffle(magicNumber int64, clients []int, matrixServices [][]*int) {
	s := rand.NewSource(magicNumber)
	ra := rand.New(s)

	ra.Shuffle(len(matrixServices), func(i, j int) {
		matrixServices[i], matrixServices[j] = matrixServices[j], matrixServices[i]
		clients[i], clients[j] = clients[j], clients[i]
	})

	ra.Shuffle(len(matrixServices[0]), func(i, j int) {
		for _, service := range matrixServices {
			service[i], service[j] = service[j], service[i]
		}
	})
}

func (r *RockSteadierSubset) AddService(ctx context.Context, ids []int) error {
	if atomic.LoadInt32(&r.close) == 1 {
		return ErrorHasBeenClosed
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r.command <- command{
		ids:  ids,
		code: 1,
	}:
		return nil
	}
}

func (r *RockSteadierSubset) addService(ids []int) {
	matrix := r.matrixServices.Load().([][]*int)
	defer r.matrixServices.Store(matrix)

	for _, id := range ids {
		matrix[r.appendIndex] = append(matrix[r.appendIndex], toPoint(id))
		x := r.appendIndex
		y := len(matrix[r.appendIndex]) - 1
		r.hasService[id] = [2]int{x, y}
		r.col = max(r.col, len(matrix[r.appendIndex]))
		r.appendIndex = (r.appendIndex + 1) % len(matrix)
	}
}

func (r *RockSteadierSubset) RemoveService(ctx context.Context, ids []int) error {
	if atomic.LoadInt32(&r.close) == 1 {
		return ErrorHasBeenClosed
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case r.command <- command{
		ids:  ids,
		code: 2,
	}:
		return nil
	}
}

func (r *RockSteadierSubset) removeService(ids []int) {
	matrix := r.matrixServices.Load().([][]*int)
	defer r.matrixServices.Store(matrix)

	for _, id := range ids {
		xy := r.hasService[id]
		matrix[xy[0]][xy[1]] = nil
		delete(r.hasService, id)
	}
}

func (r *RockSteadierSubset) GetServices(client int) []int {
	idx, ok := r.hasClient[client]
	if !ok {
		return nil
	}
	services := make([]int, 0, Lot)
	oid := idx
	matrix := r.matrixServices.Load().([][]*int)
loop:
	for (idx+1)%len(r.clients) != oid && len(services) != Lot {
		for _, v := range matrix[idx] {
			if v == nil {
				continue
			}
			services = append(services, *v)
			if len(services) == Lot {
				break loop
			}
		}
		idx = (idx + 1) % len(r.clients)
	}
	return services
}
