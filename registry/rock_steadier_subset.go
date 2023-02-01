package registry

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
)

// https://dl.acm.org/doi/10.1145/3570937

const Lot = 10

func corput(n int, base int) float64 {
	var q float64
	bk := float64(1) / float64(base)
	for n > 0 {
		q += float64(n%base) * bk
		bk /= float64(base)
		n /= base
	}
	return q
}

type RockSteadierSubset struct {
	// The number of servers in the cluster.
	Clients        []int
	matrixServices [][]*int
	appendIndex    int
	serLock        sync.RWMutex
	hasClient      map[int]int    // client: index
	hasService     map[int][2]int // service: (x,y) 快速置为nil
	col            int
}

func (r *RockSteadierSubset) matrix() string {
	b := strings.Builder{}
	b.WriteString("\t\t")
	for i := 0; i < r.col; i++ {
		b.WriteString(fmt.Sprintf("lot:%d\t", i))
	}
	b.WriteString(fmt.Sprintln())
	for idx, client := range r.Clients {
		b.WriteString(fmt.Sprintf("client:%d\t", client))
		for _, ss := range r.matrixServices[idx] {
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

func NewRockSteadierSubset(clients, services []int) *RockSteadierSubset {
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
	shuffle(pad, ls, clients, matrix)
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
	return &RockSteadierSubset{
		Clients:        clients,
		matrixServices: matrix,
		hasClient:      hasClient,
		hasService:     hasService,
		col:            col,
	}
}

func shuffle(n, base int, clients []int, matrixServices [][]*int) {
	s := rand.NewSource(int64(corput(n, base) * 10000000))
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

func (r *RockSteadierSubset) AddService(ids []int) {
	r.serLock.Lock()
	defer r.serLock.Unlock()
	for _, id := range ids {
		r.matrixServices[r.appendIndex] = append(r.matrixServices[r.appendIndex], toPoint(id))
		x := r.appendIndex
		y := len(r.matrixServices[r.appendIndex]) - 1
		r.hasService[id] = [2]int{x, y}
		r.col = max(r.col, len(r.matrixServices[r.appendIndex]))
		r.appendIndex = (r.appendIndex + 1) % len(r.matrixServices)
	}
}

func (r *RockSteadierSubset) RemoveService(ids []int) {
	r.serLock.Lock()
	defer r.serLock.Unlock()
	for _, id := range ids {
		xy := r.hasService[id]
		r.matrixServices[xy[0]][xy[1]] = nil
		delete(r.hasService, id)
	}
}

func (r *RockSteadierSubset) GetServices(client int) []int {
	r.serLock.RLock()
	defer r.serLock.RUnlock()
	idx, ok := r.hasClient[client]
	if !ok {
		return nil
	}
	services := make([]int, 0, Lot)
	oid := idx
loop:
	for (idx+1)%len(r.Clients) != oid && len(services) != Lot {
		for _, v := range r.matrixServices[idx] {
			if v == nil {
				continue
			}
			services = append(services, *v)
			if len(services) == Lot {
				break loop
			}
		}
		idx = (idx + 1) % len(r.Clients)
	}
	return services
}
