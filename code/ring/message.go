package ring

import "fmt"

type Kind int

const (
	Wakeup Kind = iota + 1
	Elect
	P1
	P2
	AnnounceLeader
)

var kindNames = map[Kind]string{Wakeup: "WAKEUP", Elect: "M", P1: "P1", P2: "P2", AnnounceLeader: "ANNOUNCE"}

func (k Kind) String() string { return kindNames[k] }

type Message struct {
	Kind  Kind
	Value int
}

func (m Message) String() string {
	switch m.Kind {
	case Wakeup:
		return "WAKEUP"
	case AnnounceLeader:
		if m.Value == 0 {
			return "SLEEPWELL"
		}
		return fmt.Sprintf("LEADER(%d)", m.Value)
	default:
		return fmt.Sprintf("%v(%d)", m.Kind, m.Value)
	}
}

type Status int

const (
	Asleep Status = iota
	Awake
	Passive
	Elected
	Done
)

func (s Status) String() string {
	return [...]string{"ASLEEP", "AWAKE", "PASSIVE", "ELECTED", "DONE"}[s]
}

type State struct {
	ID     int
	Status Status
	Leader int

	K     int
	Timer int64

	Cur  int
	Left int
	Step int
}

func (s State) String() string {
	switch {
	case s.Status == Done:
		return fmt.Sprintf("DONE leader=%d", s.Leader)
	case s.Step != 0:
		return fmt.Sprintf("%v cur=%d step=%d", s.Status, s.Cur, s.Step)
	case s.K != 0:
		return fmt.Sprintf("%v k=%d timer=%d", s.Status, s.K, s.Timer)
	default:
		return s.Status.String()
	}
}
