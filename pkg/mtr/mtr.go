package mtr

import (
	"container/ring"
	"fmt"
	"github.com/willamboss/mtr/pkg/hop"
	"github.com/willamboss/mtr/pkg/icmp"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"

	gm "github.com/buger/goterm"
)

type MTR struct {
	SrcAddress     string `json:"source"`
	mutex          *sync.RWMutex
	timeout        time.Duration
	interval       time.Duration
	Address        string `json:"destination"`
	hopsleep       time.Duration
	Statistic      map[int]*hop.HopStatistic `json:"statistic"`
	ringBufferSize int
	maxHops        int
	maxUnknownHops int
	ptrLookup      bool
}

func NewMTR(addr, srcAddr string, timeout time.Duration, interval time.Duration,
	hopsleep time.Duration, maxHops, maxUnknownHops, ringBufferSize int, ptr bool) (*MTR, chan struct{}, error) {
	if net.ParseIP(addr) == nil {
		addrs, err := net.LookupHost(addr)
		if err != nil || len(addrs) == 0 {
			return nil, nil, fmt.Errorf("invalid host or ip provided: %s", err)
		}
		addr = addrs[0]
	}
	if srcAddr == "" {
		if net.ParseIP(addr).To4() != nil {
			srcAddr = "0.0.0.0"
		} else {
			srcAddr = "::"
		}
	}
	return &MTR{
		SrcAddress:     srcAddr,
		interval:       interval,
		timeout:        timeout,
		hopsleep:       hopsleep,
		Address:        addr,
		mutex:          &sync.RWMutex{},
		Statistic:      map[int]*hop.HopStatistic{},
		maxHops:        maxHops,
		ringBufferSize: ringBufferSize,
		maxUnknownHops: maxUnknownHops,
		ptrLookup:      ptr,
	}, make(chan struct{}), nil
}

func (m *MTR) registerStatistic(ttl int, r icmp.ICMPReturn) *hop.HopStatistic {
	s, ok := m.Statistic[ttl]
	if !ok {
		s = &hop.HopStatistic{
			Sent:           0,
			TTL:            ttl,
			Timeout:        m.timeout,
			Last:           r,
			Worst:          r,
			Lost:           0,
			Packets:        ring.New(m.ringBufferSize),
			RingBufferSize: m.ringBufferSize,
		}
		m.Statistic[ttl] = s
	}

	s.Last = r
	s.Sent++

	s.Targets = addTarget(s.Targets, r.Addr)

	s.Packets = s.Packets.Prev()
	s.Packets.Value = r

	if !r.Success {
		s.Lost++
		return s // do not count failed into statistics
	}

	s.SumElapsed = r.Elapsed + s.SumElapsed

	if !s.Best.Success || s.Best.Elapsed > r.Elapsed {
		s.Best = r
	}
	if s.Worst.Elapsed < r.Elapsed {
		s.Worst = r
	}

	return s
}

func addTarget(currentTargets []string, toAdd string) []string {
	for _, t := range currentTargets {
		if t == toAdd {
			// already added
			return currentTargets
		}
	}

	var newTargets []string
	if len(currentTargets) > 0 {
		// do not add no-ip target
		if toAdd == "" {
			return currentTargets
		}

		// remove no-ip target
		for _, t := range currentTargets {
			if t != "" {
				newTargets = append(newTargets, t)
			}
		}
	} else {
		newTargets = currentTargets
	}

	// add the new one
	return append(newTargets, toAdd)
}

// TODO: aggregates everything using the first target even when there are multiple
func (m *MTR) Render(offset int) {
	gm.MoveCursor(1, offset)
	l := fmt.Sprintf("%d", m.ringBufferSize)
	gm.Printf("HOP:    %-20s  %5s%%  %4s  %6s  %6s  %6s  %6s  %"+l+"s\n", "Address", "Loss", "Sent", "Last", "Avg", "Best", "Worst", "Packets")
	for i := 1; i <= len(m.Statistic); i++ {
		gm.MoveCursor(1, offset+i)
		m.mutex.RLock()
		m.Statistic[i].Render(m.ptrLookup)
		m.mutex.RUnlock()
	}
}

// TODO: aggregates everything using the first target even when there are multiple
func (m *MTR) StringResult() []string {
	//gm.MoveCursor(1, offset)
	//fmt.Println()
	rets := []string{}
	l := fmt.Sprintf("%d", m.ringBufferSize)
	rets = append(rets, fmt.Sprintf("HOP:    %-20s  %5s%%  %4s  %6s  %6s  %6s  %6s  %"+l+"s\n", "Address", "Loss", "Sent", "Last", "Avg", "Best", "Worst", "Packets"))
	for i := 1; i <= len(m.Statistic); i++ {
		//gm.MoveCursor(1, offset+i)
		m.mutex.RLock()
		rets = append(rets, m.Statistic[i].RenderString(m.ptrLookup))
		m.mutex.RUnlock()
	}
	return rets
}

func (m *MTR) Run(ch chan struct{}, count int) {
	m.discover(ch, count)
}

// discover discovers all hops on the route
func (m *MTR) discover(ch chan struct{}, count int) {
	// Sequences are incrementing as we don't won't to get old replys which might be from a previous run (where we timed out and continued).
	// We can't use the process id as unique identifier as there might be multiple runs within a single binary, thus we use a fixed pseudo random number.
	rand.Seed(time.Now().UnixNano())
	seq := rand.Intn(math.MaxUint16)
	id := rand.Intn(math.MaxUint16) & 0xffff

	ipAddr := net.IPAddr{IP: net.ParseIP(m.Address)}

	for i := 1; i <= count; i++ {
		time.Sleep(m.interval)

		unknownHopsCount := 0
		for ttl := 1; ttl < m.maxHops; ttl++ {
			seq++
			time.Sleep(m.hopsleep)
			var hopReturn icmp.ICMPReturn
			var err error
			if ipAddr.IP.To4() != nil {
				hopReturn, err = icmp.SendDiscoverICMP(m.SrcAddress, &ipAddr, ttl, id, m.timeout, seq)
			} else {
				hopReturn, err = icmp.SendDiscoverICMPv6(m.SrcAddress, &ipAddr, ttl, id, m.timeout, seq)
			}

			m.mutex.Lock()
			s := m.registerStatistic(ttl, hopReturn)
			s.Dest = &ipAddr
			s.PID = id
			m.mutex.Unlock()
			ch <- struct{}{}
			if hopReturn.Addr == m.Address {
				break
			}
			if err != nil || !hopReturn.Success {
				unknownHopsCount++
				if unknownHopsCount >= m.maxUnknownHops {
					break
				}
				continue
			}
			unknownHopsCount = 0
		}
	}
}
