package algorithms

import (
	"archimedean/ring"
	"fmt"
)

const FCap = int64(1) << 40

type DelayFunc struct {
	Name   string
	F      func(i int) int64
	ByProc func(j, me int) int64
}

func (f DelayFunc) delay(j, me int) int64 {
	if f.ByProc != nil {
		return f.ByProc(j, me)
	}
	return f.F(j)
}

// f(i) = 2^i (at most 2N + 3N u/m messages)
var Pow2 = DelayFunc{Name: "2^i", F: func(i int) int64 { return powCapped(2, i) }}

// f(i) = (2s)^i with s = ceil(u/m) (fewer than 5N)
func Pow2S(s int64) DelayFunc {
	return DelayFunc{Name: fmt.Sprintf("(2*%d)^i", s), F: func(i int) int64 { return powCapped(2*s, i) }}
}

// f(i) = i^2 (O(N u l^a / m) messages).
var Square = DelayFunc{Name: "i^2", F: func(i int) int64 { return powCapped(int64(i), 2) }}

// f(i) = 1: Chang-Roberts with timers.
var Const = DelayFunc{Name: "1", F: func(int) int64 { return 1 }}

var MinTime = DelayFunc{Name: "2^(j-i)", ByProc: func(j, me int) int64 {
	if j >= me {
		return powCapped(2, j-me)
	}
	return 0
}}

func powCapped(b int64, e int) int64 {
	r := int64(1)
	for k := 0; k < e; k++ {
		if r > FCap/b {
			return FCap
		}
		r *= b
	}
	return r
}

func Vitanyi(f DelayFunc) ring.Algorithm { return VitanyiOpt(f, false) }

func VitanyiOpt(f DelayFunc, waitAtOrigin bool) ring.Algorithm {
	name := "Vitanyi f=" + f.Name
	if waitAtOrigin {
		name += " (wait at origin)"
	}
	return ring.Algorithm{
		Name: name,
		Wake: func(s ring.State) (ring.State, []ring.Message) {
			s.Status, s.K, s.Timer = ring.Awake, s.ID, 1
			if waitAtOrigin {
				s.Timer = f.delay(s.ID, s.ID)
			}
			return s, []ring.Message{{Kind: ring.Wakeup}}
		},
		Tick: func(s ring.State, m *ring.Message) (ring.State, []ring.Message) {
			return vitanyiTick(s, m, f)
		},
		Idle: func(s ring.State) int64 {
			if s.Status == ring.Awake && s.Timer > 0 {
				return s.Timer - 1 // these ticks only count down; the next one sends
			}
			return -1
		},
		Skip: func(s ring.State, e int64) ring.State {
			s.Timer -= e
			return s
		},
		Winner:    ring.MinID,
		OwnerWins: true,
	}
}

func vitanyiTick(s ring.State, m *ring.Message, f DelayFunc) (ring.State, []ring.Message) {
	if m != nil && m.Kind == ring.AnnounceLeader { // sleepwell
		if s.Status == ring.Elected { // it went around: the election is finished
			s.Status = ring.Done
			s.Leader = s.ID
			return s, nil
		}
		s.Status = ring.Done
		s.Leader = s.K // M_l has passed us, so k = l
		return s, []ring.Message{*m}
	}
	if m != nil && m.Kind == ring.Elect {
		switch j := m.Value; {
		case j == s.K: // our own message went around the ring (then k = i)
			s.Status = ring.Elected
			s.Leader = s.ID
			return s, []ring.Message{{Kind: ring.AnnounceLeader}}
		case j < s.K: // a smaller name: adopt it, the bigger one we held is annihilated
			s.K = j
			s.Timer = f.delay(j, s.ID)
		default: // j > k: M_j is annihilated
			s.Timer--
		}
	} else { // no message (a wakeup counts as none)
		s.Timer--
	}
	if s.Timer == 0 {
		return s, []ring.Message{{Kind: ring.Elect, Value: s.K}}
	}
	return s, nil
}
