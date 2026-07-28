package behavmatch

// Library holds named, reusable input scenarios. The seed set targets Outlaw's
// mechanics (measured this session): 4-way walk speed & clamps, and the
// fire→freeze-until-bullet-gone coupling. Each scenario is deterministic and
// ROM-agnostic — the same script drives the target and your build so any
// behavioural divergence is a numeric diff. RAM addrs are game-specific and
// optional (Outlaw: $80 P0Y, $82 P0X, $87 M0act).
var Library = map[string]Scenario{
	// Hold RIGHT after the game starts → measure P0 horizontal speed & right clamp.
	"p0-right": {
		Name: "p0-right", Frames: 130, Reset: true,
		At:      map[int][]InputChange{6: {{Action: "right", Player: 0, Press: true}}},
		Objects: []int{0}, RAM: []uint16{0x82},
	},
	// Hold LEFT → left clamp.
	"p0-left": {
		Name: "p0-left", Frames: 130, Reset: true,
		At:      map[int][]InputChange{6: {{Action: "left", Player: 0, Press: true}}},
		Objects: []int{0}, RAM: []uint16{0x82},
	},
	// Hold UP → measure P0 vertical (chunky 4px) speed & top clamp.
	"p0-up": {
		Name: "p0-up", Frames: 60, Reset: true,
		At:      map[int][]InputChange{6: {{Action: "up", Player: 0, Press: true}}},
		Objects: []int{0}, RAM: []uint16{0x80},
	},
	// Fire (press→release), then hold RIGHT → is P0 frozen while its bullet flies?
	// This is the "no Getaway" freeze==bullet-lifetime coupling.
	"p0-fire-freeze": {
		Name: "p0-fire-freeze", Frames: 70, Reset: true,
		At: map[int][]InputChange{
			4: {{Action: "fire", Player: 0, Press: true}},
			7: {{Action: "fire", Player: 0, Press: false}, {Action: "right", Player: 0, Press: true}},
		},
		Objects: []int{0, 1}, RAM: []uint16{0x82, 0x87},
	},
}

// ScenarioNames returns the library names in a stable order.
func ScenarioNames() []string {
	return []string{"p0-right", "p0-left", "p0-up", "p0-fire-freeze"}
}
