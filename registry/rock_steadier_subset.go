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
	// Start the command consumer. Without this the buffered command channel
	// filled up after cfg.buf AddService/RemoveService calls and every
	// subsequent call blocked forever — the dynamic add/remove path never ran.
	// Now that addService/removeService publish via copy-on-write, this single
	// writer is safe against concurrent GetServices readers.
	r.sentinel()
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

// addService is the sentinel-goroutine-only writer for the service matrix.
// To avoid racing with concurrent GetServices readers (which iterate the
// inner []*int slices via the atomic.Value snapshot), we copy-on-write:
// allocate a new outer slice, deep-copy any row we're about to mutate,
// and Store the new matrix at the end. The previous code mutated rows
// in place — atomic.Value only protects the outer header.
func (r *RockSteadierSubset) addService(ids []int) {
	old := r.matrixServices.Load().([][]*int)
	matrix := make([][]*int, len(old))
	copy(matrix, old)
	defer r.matrixServices.Store(matrix)

	for _, id := range ids {
		// Copy the affected row before append (which may also realloc).
		row := make([]*int, len(matrix[r.appendIndex]), len(matrix[r.appendIndex])+1)
		copy(row, matrix[r.appendIndex])
		row = append(row, toPoint(id))
		matrix[r.appendIndex] = row
		x := r.appendIndex
		y := len(row) - 1
		r.hasService[id] = [2]int{x, y}
		r.col = max(r.col, len(row))
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

// removeService applies the same copy-on-write pattern as addService.
func (r *RockSteadierSubset) removeService(ids []int) {
	old := r.matrixServices.Load().([][]*int)
	matrix := make([][]*int, len(old))
	copy(matrix, old)
	// Track which rows we've already cloned so we don't deep-copy twice
	// when multiple IDs land in the same row.
	cloned := make(map[int]bool, len(ids))
	defer r.matrixServices.Store(matrix)

	for _, id := range ids {
		xy, ok := r.hasService[id]
		if !ok {
			continue
		}
		x := xy[0]
		if !cloned[x] {
			row := make([]*int, len(matrix[x]))
			copy(row, matrix[x])
			matrix[x] = row
			cloned[x] = true
		}
		matrix[x][xy[1]] = nil
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
