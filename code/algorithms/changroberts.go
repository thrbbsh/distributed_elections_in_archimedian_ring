package algorithms

import . "archimedean/ring"

var ChangRoberts = Algorithm{
	Name: "Chang-Roberts",
	Wake: func(s State) (State, []Message) {
		s.Status = Awake
		return s, []Message{{Kind: Elect, Value: s.ID}}
	},
	Tick: func(s State, m *Message) (State, []Message) {
		if m == nil {
			return s, nil
		}
		if m.Kind == AnnounceLeader {
			return announceLeader(s, m)
		}
		switch j := m.Value; {
		case j == s.ID:
			s.Status, s.Leader = Elected, s.ID
			return s, []Message{{Kind: AnnounceLeader, Value: s.ID}}
		case j > s.ID:
			return s, []Message{*m}
		default:
			return s, nil // a smaller name dies here
		}
	},
	Idle:      func(State) int64 { return -1 },
	Skip:      func(s State, _ int64) State { return s },
	Winner:    MaxID,
	OwnerWins: true,
}

func announceLeader(s State, m *Message) (State, []Message) {
	if s.Status == Elected {
		s.Status = Done // it went around
		return s, nil
	}
	s.Status, s.Leader = Done, m.Value
	return s, []Message{*m}
}
