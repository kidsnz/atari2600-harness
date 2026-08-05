package emu

import "sort"

// covKey identifies an instruction by the bank it was FETCHED from as well as its
// address.
//
// A bare address is not an instruction on a bank-switched cartridge: both banks of
// an 8K image decode $F000-$FFFF, so bank 0's $F123 and bank 1's $F123 are two
// different instructions sharing one number. Keying on the address alone merges
// them, and it does so in both directions at once — measured 2026-07-30 over 200k
// instructions, the exerciser executes 319 distinct (bank, pc) pairs and the old
// PCCount reported 282, with 37 addresses run in BOTH banks; banked_game was 74 vs
// 69 with 5. As a count that under-reports what ran; as a query, Seen() answered
// "covered" for the twin in the bank that never ran, which is the flattering
// direction and the one VV-3's percentage and mutate's "covered" set rest on.
//
// On a flat 4K image every bank is 0, so this key changes nothing there — verified
// by diffing cmd/cover's whole JSON output before and after on flat ROMs.
type covKey struct {
	bank int
	addr uint16
}

// Coverage records the instruction addresses and branch edges a run stepped on (VV-3). It
// puts a number on the test-coverage axis = "which instruction / which branch direction did
// we actually execute". Until EnableCoverage turns it on, Emu.cov is nil = zero cost (the
// per-instruction hook is never entered).
type Coverage struct {
	pcSeen   map[covKey]bool // instructions executed (bank + start address)
	brTaken  map[covKey]bool // branch instruction → the taken edge was stepped on
	brNot    map[covKey]bool // branch instruction → the fall-through edge was stepped on
	branches map[covKey]bool // instructions found to be branches (the whole set)
}

func newCoverage() *Coverage {
	return &Coverage{
		pcSeen:   map[covKey]bool{},
		brTaken:  map[covKey]bool{},
		brNot:    map[covKey]bool{},
		branches: map[covKey]bool{},
	}
}

// record takes in one completed instruction (called from stepInstr, only on instruction
// completion). bank is the bank the instruction was FETCHED from. It must not be asked for
// after completion: an instruction that touches a hotspot changes the mapping as it
// completes, so asking afterwards attributes it to the bank it switched TO.
func (c *Coverage) record(bank int, addr uint16, isBranch, taken bool) {
	k := covKey{bank, addr}
	c.pcSeen[k] = true
	if isBranch {
		c.branches[k] = true
		if taken {
			c.brTaken[k] = true
		} else {
			c.brNot[k] = true
		}
	}
}

// PCCount is the number of distinct instructions stepped on (bank included).
func (c *Coverage) PCCount() int { return len(c.pcSeen) }

// SeenIn reports whether the instruction at that (bank, address) was executed. This is the
// only "was it stepped on" query there is.
//
// A bank-blind Seen(addr) — "executed at this address in SOME bank" — used to sit
// beside this one and was deleted 2026-07-31. It had exactly one shipped consumer,
// cmd/cover's unreached-branch loop, and there it answered in the flattering
// direction: a branch that ran only in the OTHER bank read as covered. Measured
// against the bank-aware static branch set at -frames 120 -warmup 2, 80 branches
// across the corpus were called covered which a (bank, address) comparison calls
// unreached — Pressure Cooker 35, Vanguard 19, exerciser 16, Aquaventure 9,
// banked_game 1, and 0 on litmus_bank/_f6/_f4, which are too small to collide.
// On flat 4K images the two queries are identical by construction: Chopper Command
// and Seaquest both measured 0, and cmd/cover's whole JSON output on them is
// byte-for-byte unchanged.
func (c *Coverage) SeenIn(bank int, addr uint16) bool { return c.pcSeen[covKey{bank, addr}] }

// BranchCount is the number of instructions observed to be branches (bank included).
func (c *Coverage) BranchCount() int { return len(c.branches) }

// dedupeAddrs collapses a (bank,addr) set into an ascending address list (to keep the shape
// of the outward-facing API).
func dedupeAddrs(keys map[covKey]bool) []uint16 {
	seen := map[uint16]bool{}
	out := make([]uint16, 0, len(keys))
	for k := range keys {
		if !seen[k.addr] {
			seen[k.addr] = true
			out = append(out, k.addr)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// sortedSites turns a (bank,addr) set into a list of (bank, address) pairs, ascending.
func sortedSites(keys map[covKey]bool) [][2]int {
	out := make([][2]int, 0, len(keys))
	for k := range keys {
		out = append(out, [2]int{k.bank, int(k.addr)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i][0] != out[j][0] {
			return out[i][0] < out[j][0]
		}
		return out[i][1] < out[j][1]
	})
	return out
}

// BranchSites is every instruction observed to BE a branch, as (bank, address)
// pairs ascending. It is the observed side of the coverage report: a site in here
// and not in the static decode is code the decoder never reached, which means the
// static denominator is too small.
//
// It replaced BranchAddrs() []uint16 on 2026-07-31, because the static side it is
// compared against is now keyed on (bank, address) and a bare address cannot be
// looked up in that set at all. This is not a number that moved: measured over the
// corpus at -frames 120, dedupeAddrs merged observed branch sites on exactly one
// image (exerciser, 14 sites to 13 addresses) and the executed-but-undecoded count
// came out identical under either keying — 0 everywhere except Aquaventure, which
// reports 14 both ways. It is the comparison that was impossible, not the answer
// that was wrong, and the merge becomes wrong the moment a run enters one branch
// address in two banks. There is no bank-blind branch query left.
func (c *Coverage) BranchSites() [][2]int { return sortedSites(c.branches) }

// EdgeCount is the total number of branch edges stepped on (max = BranchCount*2 = both
// directions covered).
func (c *Coverage) EdgeCount() int { return len(c.brTaken) + len(c.brNot) }

// OneSidedBranchSites returns the branches where only one edge was stepped on, as (bank,
// address) ascending (taken only / not only = a sign that the other side of that branch is
// untested).
//
// Bank-aware since 2026-07-31. The address-only OneSidedBranches() it replaced
// reported "one-sided in SOME bank", which merged a fully exercised branch in one
// bank with a half-exercised twin in another and named neither — and it put an
// address-only list in the same JSON object as a bank-qualified unreached list,
// where the two read as comparable sets and are not.
func (c *Coverage) OneSidedBranchSites() [][2]int {
	oneSided := map[covKey]bool{}
	for k := range c.branches {
		if c.brTaken[k] != c.brNot[k] { // exactly one of the two is true
			oneSided[k] = true
		}
	}
	return sortedSites(oneSided)
}

// Signature returns the "coverage markers" stepped on as a comparable key set, for
// coverage-guided fuzz feedback (instruction address + branch edge taken/not). An input that
// adds a new marker = interesting. The bank is included, so on an 8K image the same address
// in a different bank is counted as a different marker (without it, having walked only one
// of the banks looks like "no new coverage" and the search stalls).
func (c *Coverage) Signature() map[uint64]bool {
	sig := make(map[uint64]bool, len(c.pcSeen)+len(c.brTaken)+len(c.brNot))
	mark := func(tag uint64, k covKey, edge uint64) uint64 {
		return tag | uint64(k.bank)<<40 | uint64(k.addr)<<1 | edge
	}
	for k := range c.pcSeen {
		sig[mark(0, k, 0)] = true
	}
	for k := range c.brTaken {
		sig[mark(1<<32, k, 1)] = true
	}
	for k := range c.brNot {
		sig[mark(1<<32, k, 0)] = true
	}
	return sig
}

// SeenSites returns the instructions stepped on as (bank, address) pairs, ascending. A
// caller that wants to convert these into offsets inside the ROM file must use this one: the
// address alone does not decide which 4K face of an 8K image it is, and
// `addr & (len(rom)-1)` always folds $Fxxx onto the upper face. Measured 2026-07-30: under
// that folding all 278 covered offsets of the exerciser landed in the upper 4K, and not one
// byte of bank 0 was picked.
func (c *Coverage) SeenSites() [][2]int { return sortedSites(c.pcSeen) }

// SeenPCs returns the instruction addresses stepped on, ascending (for dumps).
func (c *Coverage) SeenPCs() []uint16 { return dedupeAddrs(c.pcSeen) }

// Reset empties the record (so guidedfuzz can reuse one machine instead of reloading for
// every evaluation; signature identity was verified first, and at warmup=200 it is about
// 100x faster).
func (c *Coverage) Reset() {
	c.pcSeen = map[covKey]bool{}
	c.brTaken = map[covKey]bool{}
	c.brNot = map[covKey]bool{}
	c.branches = map[covKey]bool{}
}
