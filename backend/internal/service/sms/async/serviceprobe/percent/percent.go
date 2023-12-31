package percent

import (
	"context"
	"sync"

	"github.com/johnwongx/webook/backend/internal/service/sms/async/serviceprobe"
)

type Percent struct {
	size        int
	index       int
	molecular   int
	isFilled    bool
	arr         []error
	isMolecular func(val error) bool
	threshod    float64

	mu sync.Mutex
}

func NewPercent(size int, isMolecular func(err error) bool, threshod float64) serviceprobe.ServiceProbe {
	return &Percent{
		size:        size,
		arr:         make([]error, size),
		isMolecular: isMolecular,
		threshod:    threshod,
	}
}

func (p *Percent) Add(ctx context.Context, err error) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isFilled && p.isMolecular(p.arr[p.index]) {
		p.molecular--
	}

	p.arr[p.index] = err
	p.index = (p.index + 1) % p.size
	if p.isMolecular(err) {
		p.molecular++
	}
	if p.index == 0 && !p.isFilled {
		p.isFilled = true
	}

	return p.isCrashedWithoutLock(ctx)
}

func (p *Percent) IsCrashed(ctx context.Context) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.isCrashedWithoutLock(ctx)
}

func (p *Percent) isCrashedWithoutLock(ctx context.Context) bool {
	var res float64
	if p.isFilled {
		res = float64(p.molecular) / float64(p.size)
	} else {
		res = float64(p.molecular) / float64(p.index)
	}
	return res > p.threshod
}
