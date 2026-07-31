package cyclebound

// The edge-semantics whitelist is a claim about SOMEONE ELSE'S SOURCE, and until now
// the only thing holding it up was that a human had read that source once.
//
// `verifiedEdgeSemantics` says a mapper's bank-switch is selected by the ADDRESS
// ALONE, and each entry cites the file and method where that was read. The citation
// is prose: nothing checked that the file exists, that the method exists in it, or —
// the part that actually matters — that the method still selects the bank from the
// address. Gopher2600 is a `replace ./Gopher2600` dependency that gets updated; a
// mapper whose switch grows a data-bus condition would silently invalidate an entry,
// and the analysis would keep emitting cross-bank edges the hardware never takes.
//
// WF8 is the proof that this failure mode is real rather than theoretical: it
// publishes $1FF8:BANK0 / $1FF9:BANK1 like an Atari mapper and then takes the target
// bank from DATA BUS BIT 2. Reading its hotspot table and concluding "address only"
// is exactly the mistake the whitelist would make. It was caught by hand.
//
// This turns the citation into a machine-checked invariant: parse the cited method
// out of the cited file with go/ast and require that a mapper claimed as address-only
// does not consult the data bus. The two directions are asserted together, so the
// check cannot pass by matching nothing:
//
//   verified   -> the cited bankswitch MUST NOT reference its data parameter
//   different  -> the cited bankswitch MUST reference it (WF8's does; FA's does)
//
// WHAT THIS PROVES, EXACTLY. That a whitelisted mapper's bank selection is not a
// function of the data bus. That is one failure mode, not all of them: E0 switches
// three 1K SEGMENTS rather than a whole-window bank, E7 spends four of its hotspots
// on RAM instead of banks, FA2 makes its last hotspot conditional on the image size.
// None of those touch the data bus, so this gate is blind to them and they are
// recorded in `knownDifferentEdgeSemantics` in prose instead. A gate that covers one
// failure mode is worth having as long as it does not get read as covering the rest.
//
// There is no skip path. Gopher2600 is a local `replace` target, so a tree where
// these files are missing is a tree where nothing compiles.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// gopherCartDir is where the mapper implementations live, relative to this package.
const gopherCartDir = "../../Gopher2600/hardware/memory/cartridge"

// citation is "<file>.go <receiver>.bankswitch", the form every whitelist entry's
// evidence string opens with.
var citationRE = regexp.MustCompile(`(mapper_[a-z0-9_]+\.go) ([a-zA-Z0-9]+)\.bankswitch`)

type citation struct {
	file     string
	receiver string
}

// parseCitation pulls the file and receiver out of an evidence string. ok is false
// when the string carries no citation at all (some entries are cross-references to
// another entry rather than a fresh reading).
func parseCitation(evidence string) (citation, bool) {
	m := citationRE.FindStringSubmatch(evidence)
	if m == nil {
		return citation{}, false
	}
	return citation{file: m[1], receiver: m[2]}, true
}

// bankswitchUsesDataBus finds `func (x *<receiver>) bankswitch(...)` in the given
// file and reports whether its body reads a data-bus parameter.
//
// Two ways to be address-only, and both are accepted: the method may not take a data
// argument at all (the Atari family), or it may take one and never read it (JANE
// takes `data uint8` to satisfy a shared call site and ignores it). The distinction
// matters because a signature check alone would reject JANE, and a body-only check
// would have nothing to look for in the others.
//
// The body is walked as an AST rather than searched as text: CBS's bankswitch quotes
// the patent in a comment that says "data line D0" six times, and a grep for "data"
// would score that as a use.
func bankswitchUsesDataBus(t *testing.T, file, receiver string) bool {
	t.Helper()

	path := filepath.Join(gopherCartDir, file)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("cited source %s does not exist: %v\n"+
			"The evidence string in the edge-semantics table names a file that is not there, so the "+
			"claim it carries cannot be checked at all.", path, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0) // mode 0: comments are not attached
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "bankswitch" || fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}
		if recvTypeName(fn.Recv.List[0].Type) != receiver {
			continue
		}

		// Collect the names of every parameter after the address. Anything the
		// mapper is handed besides the address is the data bus.
		dataParams := map[string]bool{}
		for i, p := range fn.Type.Params.List {
			if i == 0 {
				continue // addr
			}
			for _, n := range p.Names {
				if n.Name != "_" {
					dataParams[n.Name] = true
				}
			}
		}
		if len(dataParams) == 0 {
			return false // no data argument to consult
		}

		used := false
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && dataParams[id.Name] {
				used = true
			}
			return !used
		})
		return used
	}

	t.Fatalf("cited method %s.bankswitch not found in %s\n"+
		"The evidence string names a method that no longer exists, which means the reading it records "+
		"was of a different version of the mapper.", receiver, file)
	return false
}

func recvTypeName(e ast.Expr) string {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// TestVerifiedEdgeSemanticsCitationsAreAddressOnly re-reads, mechanically, the source
// each whitelist entry says it read.
func TestVerifiedEdgeSemanticsCitationsAreAddressOnly(t *testing.T) {
	if len(verifiedEdgeSemantics) == 0 {
		t.Fatal("verifiedEdgeSemantics is empty; this check would assert nothing")
	}
	for id, evidence := range verifiedEdgeSemantics {
		c, ok := parseCitation(evidence)
		if !ok {
			t.Errorf("%s: evidence cites no source (%q)\n"+
				"An entry on the address-only whitelist has to say where that was read, or it is a "+
				"label rather than evidence.", id, evidence)
			continue
		}
		if bankswitchUsesDataBus(t, c.file, c.receiver) {
			t.Errorf("%s is whitelisted as address-only, but %s %s.bankswitch reads the data bus\n"+
				"A cross-bank edge derived from the hotspot symbol alone would be an edge the hardware "+
				"does not take. Move this entry to knownDifferentEdgeSemantics.",
				id, c.file, c.receiver)
		}
	}
}

// TestKnownDifferentDataBusCitationsDoUseIt is the other direction, and it is what
// keeps the check above from passing vacuously: the mappers recorded as data-driven
// have to actually be data-driven in the same source, read by the same parser.
//
// Only the entries whose stated reason IS the data bus are asserted on. E0, E7 and
// FA2 differ for structural reasons the parser cannot see, so requiring a data-bus
// read from them would be requiring the wrong thing.
func TestKnownDifferentDataBusCitationsDoUseIt(t *testing.T) {
	asserted := 0
	for id, reason := range knownDifferentEdgeSemantics {
		if !strings.Contains(reason, "DATA BUS") && !strings.Contains(reason, "data bus") {
			continue
		}
		c, ok := parseCitation(reason)
		if !ok {
			continue // cross-reference to another entry, e.g. WFSC -> WF8
		}
		if !bankswitchUsesDataBus(t, c.file, c.receiver) {
			t.Errorf("%s is recorded as data-bus driven, but %s %s.bankswitch never reads the data bus\n"+
				"Either the record is wrong or the detector is: both readings come from the same parser, "+
				"so one of them is not measuring what it claims.", id, c.file, c.receiver)
		}
		asserted++
	}
	if asserted == 0 {
		t.Fatal("no data-bus-driven mapper was asserted on, so the address-only check above has no " +
			"witness that its detector can fire at all")
	}
	t.Logf("data-bus-driven mappers re-read from source: %d", asserted)
}

// TestEdgeSemanticsTablesAreDisjoint keeps a mapper from being both checked-and-fine
// and checked-and-broken. unverifiedEdgeSemantics consults the whitelist first, so an
// ID in both would silently take the permissive answer.
func TestEdgeSemanticsTablesAreDisjoint(t *testing.T) {
	for id := range verifiedEdgeSemantics {
		if _, bad := knownDifferentEdgeSemantics[id]; bad {
			t.Errorf("%s is in both verifiedEdgeSemantics and knownDifferentEdgeSemantics; the "+
				"whitelist is consulted first, so the mapper would be treated as modelled", id)
		}
	}
}
