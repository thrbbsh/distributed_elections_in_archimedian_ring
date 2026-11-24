package ring

import (
	"container/heap"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type turnKind int

const (
	turnWake turnKind = iota
	turnTick
)

type turn struct {
	kind turnKind
	now  int64
	read bool
}

type report struct {
	read          *Message
	before, after State
	sent          []Message
	idle          int64
}

type node struct {
	algo   Algorithm
	state  State
	period int64
	last   int64
	inbox  []Message
	in     <-chan Message
	out    chan<- Message
	turn   chan turn
	done   chan<- report
}

func (v *node) run(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case m := <-v.in:
			v.inbox = append(v.inbox, m)
		case t, ok := <-v.turn:
			if !ok {
				return
			}
			v.step(t)
		}
	}
}

func (v *node) step(t turn) {
	v.drainLink()
	r := report{before: v.state}
	switch t.kind {
	case turnWake:
		v.state, r.sent = v.algo.Wake(v.state)
	case turnTick:
		if e := (t.now-v.last)/v.period - 1; e > 0 {
			v.state = v.algo.Skip(v.state, e)
		}
		if t.read {
			m := v.inbox[0]
			v.inbox = v.inbox[1:]
			r.read = &m
		}
		v.state, r.sent = v.algo.Tick(v.state, r.read)
	}
	v.last = t.now
	for _, m := range r.sent {
		v.send(m)
	}
	r.after, r.idle = v.state, v.algo.Idle(v.state)
	v.done <- r
}

func (v *node) drainLink() {
	for {
		select {
		case m := <-v.in:
			v.inbox = append(v.inbox, m)
		default:
			return
		}
	}
}

func (v *node) send(m Message) {
	for {
		select {
		case v.out <- m:
			return
		case x := <-v.in:
			v.inbox = append(v.inbox, x)
		}
	}
}

type evKind int

const (
	evArrive evKind = iota
	evWake
	evTick
)

type event struct {
	time int64
	kind evKind
	seq  int64
	pos  int
	msg  Message
	ver  int
}

type eventQueue []event

func (q eventQueue) Len() int { return len(q) }
func (q eventQueue) Less(a, b int) bool {
	if q[a].time != q[b].time {
		return q[a].time < q[b].time
	}
	if q[a].kind != q[b].kind {
		return q[a].kind < q[b].kind
	}
	return q[a].seq < q[b].seq
}
func (q eventQueue) Swap(a, b int) { q[a], q[b] = q[b], q[a] }
func (q *eventQueue) Push(x any)   { *q = append(*q, x.(event)) }
func (q *eventQueue) Pop() any {
	old := *q
	e := old[len(old)-1]
	*q = old[:len(old)-1]
	return e
}

type clock struct {
	id           int
	status       Status
	period       int64
	last         int64
	next         int64
	ver          int
	arrived      int
	lastDelivery int64
}

type scheduler struct {
	algo   Algorithm
	cfg    Config
	rng    *rand.Rand
	nodes  []*node
	clocks []*clock
	done   chan report
	q      eventQueue
	seq    int64
	now    int64
	res    Result
}

func Run(algo Algorithm, cfg Config, rng *rand.Rand) Result {
	n := len(cfg.IDs)
	s := &scheduler{algo: algo, cfg: cfg, rng: rng, done: make(chan report)}
	s.res = Result{Algo: algo.Name, IDs: cfg.IDs, Winner: algo.Winner(cfg.IDs), LeaderPos: -1,
		OwnerWins: algo.OwnerWins, Count: map[Kind]int{}}
	links := make([]chan Message, n)
	for i := range links {
		links[i] = make(chan Message, 1)
	}
	minP, maxP := int64(-1), int64(0)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		p := cfg.TickMin
		if cfg.Periods != nil {
			p = cfg.Periods[i]
		} else if cfg.TickMax > cfg.TickMin {
			p += rng.Int63n(cfg.TickMax - cfg.TickMin + 1)
		}
		if p < 1 {
			panic("clock periods must be at least 1")
		}
		if minP < 0 || p < minP {
			minP = p
		}
		maxP = max(maxP, p)
		s.res.Periods = append(s.res.Periods, p)
		v := &node{algo: algo, state: State{ID: cfg.IDs[i]}, period: p, in: links[i], out: links[(i+1)%n],
			turn: make(chan turn), done: s.done}
		s.nodes = append(s.nodes, v)
		s.clocks = append(s.clocks, &clock{id: cfg.IDs[i], period: p, next: -1})
		wg.Add(1)
		go v.run(&wg)
	}
	s.res.U, s.res.M = maxP+cfg.DelayMax, minP
	for i := 0; i < n; i++ {
		at := int64(0)
		if cfg.WakeAt != nil {
			at = cfg.WakeAt[i]
		}
		if at >= 0 {
			s.push(event{time: at, kind: evWake, pos: i})
		}
	}
	s.loop()
	for _, v := range s.nodes {
		close(v.turn)
	}
	wg.Wait()
	for _, v := range s.nodes {
		s.res.Final = append(s.res.Final, v.state)
	}
	return s.res
}

func (s *scheduler) loop() {
	maxEvents := s.cfg.MaxEvents
	if maxEvents == 0 {
		maxEvents = 100_000_000
	}
	for s.q.Len() > 0 {
		e := heap.Pop(&s.q).(event)
		c := s.clocks[e.pos]
		if e.kind == evTick && e.ver != c.ver {
			continue
		}
		s.res.Events++
		if s.res.Events > maxEvents {
			panic(fmt.Sprintf("more than %d events: the run does not stop", maxEvents))
		}
		s.now = e.time
		s.res.Time = s.now
		switch e.kind {
		case evWake:
			if c.status == Asleep {
				s.wake(e.pos, "spontaneously")
			}
		case evArrive:
			s.arrive(e.pos, e.msg)
		case evTick:
			s.tick(e.pos)
		}
	}
}

func (s *scheduler) push(e event) {
	s.seq++
	e.seq = s.seq
	heap.Push(&s.q, e)
}

func (s *scheduler) step(pos int, t turn) report {
	s.nodes[pos].turn <- t
	r := <-s.done
	c := s.clocks[pos]
	c.status, c.last = r.after.Status, s.now
	return r
}

func (s *scheduler) wake(pos int, why string) {
	r := s.step(pos, turn{kind: turnWake, now: s.now})
	logf("t=%9d  P%d (id %d) wakes up %s%*s-> %v", s.now, pos, r.after.ID, why, 22-len(why), "", r.after)
	s.deliver(pos, r.sent)
	s.schedule(pos, r.idle)
}

func (s *scheduler) arrive(pos int, m Message) {
	c := s.clocks[pos]
	if c.status == Done {
		s.res.Stray++
		logf("t=%9d  P%d (id %d) is done, ignores late %v", s.now, pos, c.id, m)
		return
	}
	c.arrived++
	if c.status == Asleep {
		s.wake(pos, "by "+m.String())
		return
	}
	if c.arrived > 1 {
		return
	}
	k := (s.now - c.last + c.period - 1) / c.period
	if k < 1 {
		k = 1
	}
	if t := c.last + k*c.period; c.next < 0 || t < c.next {
		s.setTick(pos, t)
	}
}

func (s *scheduler) tick(pos int) {
	c := s.clocks[pos]
	c.next = -1
	read := c.arrived > 0
	if read {
		c.arrived--
	}
	r := s.step(pos, turn{kind: turnTick, now: s.now, read: read})
	if r.read != nil || len(r.sent) > 0 || r.before.Status != r.after.Status {
		s.res.Ticks++
		time.Sleep(Delay)
		got := "(no message)"
		if r.read != nil {
			got = "got " + r.read.String()
		}
		logf("t=%9d  P%d (id %d) %-30v -> %v", s.now, pos, r.after.ID, got, r.after)
	}
	if r.after.Status == Elected && r.before.Status != Elected {
		s.res.Elected++
		s.res.LeaderPos, s.res.LeaderValue = pos, r.after.Leader
		logf("           *** P%d (id %d) is elected with value %d ***", pos, r.after.ID, r.after.Leader)
	}
	s.deliver(pos, r.sent)
	s.schedule(pos, r.idle)
}

func (s *scheduler) deliver(pos int, msgs []Message) {
	c := s.clocks[pos]
	to := (pos + 1) % len(s.clocks)
	for _, m := range msgs {
		d := s.cfg.DelayMin
		if s.cfg.DelayMax > s.cfg.DelayMin {
			d += s.rng.Int63n(s.cfg.DelayMax - s.cfg.DelayMin + 1)
		}
		t := max(s.now+d, c.lastDelivery)
		c.lastDelivery = t
		s.res.Count[m.Kind]++
		s.res.Bits += Bits(m)
		logf("           P%d sent %v to P%d (arrives t=%d)", pos, m, to, t)
		s.push(event{time: t, kind: evArrive, pos: to, msg: m})
	}
}

func (s *scheduler) schedule(pos int, idle int64) {
	c := s.clocks[pos]
	switch {
	case c.status == Done:
		s.cancelTick(pos)
	case c.arrived > 0:
		s.setTick(pos, c.last+c.period)
	case idle < 0:
		s.cancelTick(pos)
	default:
		s.setTick(pos, c.last+(idle+1)*c.period)
	}
}

func (s *scheduler) setTick(pos int, t int64) {
	c := s.clocks[pos]
	c.ver++
	c.next = t
	s.push(event{time: t, kind: evTick, pos: pos, ver: c.ver})
}

func (s *scheduler) cancelTick(pos int) {
	c := s.clocks[pos]
	c.ver++
	c.next = -1
}
