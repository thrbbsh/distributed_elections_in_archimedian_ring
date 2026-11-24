package algorithms

import . "archimedean/ring"

var Peterson = Algorithm{
	Name: "Peterson",
	Wake: func(s State) (State, []Message) {
		s.Status, s.Cur, s.Step = Awake, s.ID, 1
		return s, []Message{{Kind: P1, Value: s.Cur}}
	},
	Tick: func(s State, m *Message) (State, []Message) {
		if m == nil {
			return s, nil
		}
		if m.Kind == AnnounceLeader {
			return announceLeader(s, m)
		}
		if s.Status != Awake { // passive: relay
			return s, []Message{*m}
		}
		switch {
		case s.Step == 1 && m.Kind == P1:
			if m.Value == s.Cur { // it's the only active process
				s.Status, s.Leader = Elected, s.Cur
				return s, []Message{{Kind: AnnounceLeader, Value: s.Cur}}
			}
			s.Left, s.Step = m.Value, 2
			return s, []Message{{Kind: P2, Value: m.Value}}
		case s.Step == 2 && m.Kind == P2:
			if s.Left > m.Value && s.Left > s.Cur { // the left value is a local maximum: take it over
				s.Cur, s.Step = s.Left, 1
				return s, []Message{{Kind: P1, Value: s.Cur}}
			}
			s.Status = Passive
			return s, nil
		}
		panic("Peterson: message out of order, links are not FIFO")
	},
	Idle:   func(State) int64 { return -1 },
	Skip:   func(s State, _ int64) State { return s },
	Winner: MaxID,
}
