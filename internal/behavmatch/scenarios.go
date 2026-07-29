package behavmatch

// Library holds named, reusable input scenarios.
//
// Every scenario is a deterministic, ROM-AGNOSTIC input script: the same script
// drives the target and your build, so any behavioural difference is a numeric
// diff rather than a hunch. Nothing here names a game's variables — all 128 RAM
// bytes are recorded unconditionally, because which addresses matter is precisely
// what is being measured. (An earlier version declared Outlaw addresses like
// $82; measuring the real cartridge showed those were guesses from our own
// reconstruction and wrong for the actual ROM, which is exactly the mistake a
// ROM-agnostic script cannot make.)
//
// What the suite has to cover, and why each entry exists:
//
//   - Every direction for EACH player. A suite that only drives P0 leaves the
//     mirrored half of a two-player game unmeasured, and a sign or bound error in
//     it passes silently.
//   - Fire as a TAP and as a HOLD. Press-edge, release-edge and level-triggered
//     firing are indistinguishable if the button is only ever held one way.
//   - Both players firing at once, so any coupling that only exists when two
//     things are in flight in the same frame is exercised at all.
//   - Fire while moving, and fire in each aimed direction, because a spawn that
//     reads the walker's state is a different rule from one that does not.
//   - A long free-running exchange, so slow state (scores, timers, wins) is
//     reached by playing rather than by poking values in.
var Library = map[string]Scenario{}

// order is the stable presentation order; ScenarioNames returns it.
var order []string

func add(s Scenario) {
	Library[s.Name] = s
	order = append(order, s.Name)
}

// press/release helpers keep the scripts readable as intent rather than as maps.
func press(player int, action string) InputChange {
	return InputChange{Action: action, Player: player, Press: true}
}

func release(player int, action string) InputChange {
	return InputChange{Action: action, Player: player, Press: false}
}

// startFrame is when scripted input begins: late enough that the RESET press has
// been released and the game has settled into play.
const startFrame = 6

func init() {
	// --- one direction held, per player -------------------------------------
	// Speed, cadence and the bound in that direction. Held long enough that the
	// walker reaches and sits at its limit, since a clamp that is never reached
	// is a clamp that was never measured.
	for _, p := range []int{0, 1} {
		for _, dir := range []string{"right", "left", "up", "down"} {
			add(Scenario{
				Name: name(p, dir), Frames: 150, Reset: true,
				At:      map[int][]InputChange{startFrame: {press(p, dir)}},
				Objects: []int{0, 1, 2, 3},
			})
		}
	}

	// --- diagonals ----------------------------------------------------------
	// Two axes at once: catches an update that services only one axis per frame,
	// or that couples them.
	add(Scenario{
		Name: "p0-up-right", Frames: 120, Reset: true,
		At:      map[int][]InputChange{startFrame: {press(0, "up"), press(0, "right")}},
		Objects: []int{0, 1, 2, 3},
	})
	add(Scenario{
		Name: "p1-down-left", Frames: 120, Reset: true,
		At:      map[int][]InputChange{startFrame: {press(1, "down"), press(1, "left")}},
		Objects: []int{0, 1, 2, 3},
	})

	// --- fire: tap vs hold, per player --------------------------------------
	// The pair is the point. A tap and a hold produce the same trace under a
	// press-edge rule and different traces under a release-edge or level rule, so
	// running only one of them cannot tell those rules apart (G3/G10).
	for _, p := range []int{0, 1} {
		add(Scenario{
			Name: nameFire(p, "tap"), Frames: 120, Reset: true,
			At: map[int][]InputChange{
				startFrame:     {press(p, "fire")},
				startFrame + 2: {release(p, "fire")},
			},
			Objects: []int{0, 1, 2, 3},
		})
		add(Scenario{
			Name: nameFire(p, "hold"), Frames: 120, Reset: true,
			At:      map[int][]InputChange{startFrame: {press(p, "fire")}},
			Objects: []int{0, 1, 2, 3},
		})
	}

	// --- fire then move: is the shooter frozen while its shot is out? --------
	// Holding a direction immediately after firing is what makes a freeze visible:
	// if the walker does not move while the shot lives, something is gating it.
	add(Scenario{
		Name: "p0-fire-freeze", Frames: 120, Reset: true,
		At: map[int][]InputChange{
			startFrame:     {press(0, "fire")},
			startFrame + 3: {release(0, "fire"), press(0, "right")},
		},
		Objects: []int{0, 1, 2, 3},
	})
	add(Scenario{
		Name: "p1-fire-freeze", Frames: 120, Reset: true,
		At: map[int][]InputChange{
			startFrame:     {press(1, "fire")},
			startFrame + 3: {release(1, "fire"), press(1, "left")},
		},
		Objects: []int{0, 1, 2, 3},
	})

	// --- aimed fire ---------------------------------------------------------
	// Direction held at the moment of firing. If a shot's trajectory is set from
	// the shooter's facing, this is where that shows up.
	for _, p := range []int{0, 1} {
		for _, dir := range []string{"up", "down"} {
			add(Scenario{
				Name: nameFire(p, "aim-"+dir), Frames: 120, Reset: true,
				At: map[int][]InputChange{
					startFrame:      {press(p, dir)},
					startFrame + 8:  {press(p, "fire")},
					startFrame + 10: {release(p, "fire")},
				},
				Objects: []int{0, 1, 2, 3},
			})
		}
	}

	// --- both players firing in the same frame ------------------------------
	// Nothing else in the suite puts two shots in the air simultaneously, so any
	// interaction between them is otherwise untested.
	add(Scenario{
		Name: "both-fire", Frames: 150, Reset: true,
		At: map[int][]InputChange{
			startFrame:     {press(0, "fire"), press(1, "fire")},
			startFrame + 2: {release(0, "fire"), release(1, "fire")},
		},
		Objects: []int{0, 1, 2, 3},
	})

	// --- players walking toward each other and firing repeatedly ------------
	// The only way to reach slow state — scores, timers, win conditions — by
	// playing rather than by poking a value in. Long, and deliberately so.
	add(Scenario{Name: "duel-long", Frames: 900, Reset: true,
		At: duelScript(), Objects: []int{0, 1, 2, 3}})

	// --- console switches ---------------------------------------------------
	// Whether the game has an attract mode, a select-cycled variant, or a mid-play
	// reset is a property of the ROM, not something to assume. This exercises it.
	add(Scenario{
		Name: "idle-no-input", Frames: 400, Reset: false,
		Objects: []int{0, 1, 2, 3},
	})
	add(Scenario{
		Name: "select-then-reset", Frames: 200, Reset: false,
		At: map[int][]InputChange{
			10: {{Panel: "select", Press: true}},
			18: {{Panel: "select", Press: false}},
			40: {{Panel: "reset", Press: true}},
			48: {{Panel: "reset", Press: false}},
		},
		Objects: []int{0, 1, 2, 3},
	})
}

// duelScript walks the two players together and has each fire on a different
// cadence, so shots overlap irregularly instead of in lockstep.
func duelScript() map[int][]InputChange {
	at := map[int][]InputChange{
		startFrame: {press(0, "right"), press(1, "left")},
	}
	for f := startFrame + 20; f < 880; f += 26 {
		at[f] = append(at[f], press(0, "fire"))
		at[f+3] = append(at[f+3], release(0, "fire"))
	}
	for f := startFrame + 33; f < 880; f += 31 {
		at[f] = append(at[f], press(1, "fire"))
		at[f+4] = append(at[f+4], release(1, "fire"))
	}
	// Reverse direction part-way so both walkers visit both halves of their range
	// and the bounds at both ends are reached.
	at[300] = append(at[300], release(0, "right"), release(1, "left"),
		press(0, "left"), press(1, "right"))
	at[600] = append(at[600], release(0, "left"), release(1, "right"),
		press(0, "up"), press(1, "down"))
	return at
}

func name(player int, dir string) string {
	return playerTag(player) + "-" + dir
}

func nameFire(player int, suffix string) string {
	return playerTag(player) + "-fire-" + suffix
}

func playerTag(player int) string {
	if player == 1 {
		return "p1"
	}
	return "p0"
}

// ScenarioNames returns the library names in a stable order.
func ScenarioNames() []string { return append([]string(nil), order...) }
