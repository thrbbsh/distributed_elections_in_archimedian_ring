package ring

import (
	"fmt"
	"time"
)

type Algorithm struct {
	Name      string
	Wake      func(State) (State, []Message)               // WAKEUP
	Tick      func(s State, m *Message) (State, []Message) // One tick with a message (nil if none)
	Idle      func(State) int64                            // Only for Vitanyi: how many ticks until the next message is sent (0 = now, -1 = never)
	Skip      func(State, int64) State                     // Only for Vitanyi: skip e ticks (e > 0)
	Winner    func(ids []int) int                          // The name that must win (max or min)
	OwnerWins bool                                         // true if the owner of the winning name must win (Vitanyi, Chang-Roberts), false if any process with that name may win (Peterson)
}

type Config struct {
	IDs                []int   // names in ring
	TickMin, TickMax   int64   // range of clock periods (absolute time units)
	Periods            []int64 // optional: explicit clock period per position (overrides TickMin/TickMax)
	DelayMin, DelayMax int64   // range of link delays
	WakeAt             []int64 // spontaneous wake-up time per position, -1 = never spontaneously; nil = all at time 0
	MaxEvents          int     // safety stop (0 = 100 million)
}

var Delay time.Duration
var ShowLogs = true

func logf(format string, args ...any) {
	if ShowLogs {
		fmt.Printf(format+"\n", args...)
	}
}

type Result struct {
	Algo        string
	IDs         []int
	Winner      int // the name that had to win
	LeaderPos   int // position that became ELECTED (-1 if none)
	LeaderValue int // the value it was elected with
	OwnerWins   bool
	Elected     int          // how many processes became ELECTED
	Final       []State      // states at the end
	Count       map[Kind]int // messages sent, by kind
	Bits        int          // bits sent (wakeup and sleepwell: 1 bit)
	Time        int64        // absolute time of the last useful event
	Stray       int          // messages that arrived at a process that was already DONE
	Ticks       int          // simulated ticks with an action or a message
	Events      int          // processed events
	U, M        int64
	Periods     []int64 // the clock periods that were used
}

func (r Result) Total() int {
	t := 0
	for _, c := range r.Count {
		t += c
	}
	return t
}

func (r Result) Election() int { return r.Total() - r.Count[Wakeup] - r.Count[AnnounceLeader] }

func (r Result) Check() error {
	if r.Elected != 1 {
		return fmt.Errorf("%d processes were elected", r.Elected)
	}
	if r.LeaderValue != r.Winner {
		return fmt.Errorf("P%d was elected with value %d, but %d must win", r.LeaderPos, r.LeaderValue, r.Winner)
	}
	if r.OwnerWins && r.IDs[r.LeaderPos] != r.Winner {
		return fmt.Errorf("P%d (id %d) was elected, but the owner of %d must win", r.LeaderPos, r.IDs[r.LeaderPos], r.Winner)
	}
	for i, s := range r.Final {
		if s.Status != Done || s.Leader != r.Winner {
			return fmt.Errorf("P%d ends in %v and believes the leader is %d (want %d)", i, s.Status, s.Leader, r.Winner)
		}
	}
	if r.Stray != 0 {
		return fmt.Errorf("%d messages arrived after the election was over", r.Stray)
	}
	return nil
}

func Bits(m Message) int {
	if m.Kind == Wakeup || m.Value == 0 {
		return 1
	}
	d := 0
	for v := m.Value + 1; v > 1; v >>= 1 {
		d++
	}
	return d
}
