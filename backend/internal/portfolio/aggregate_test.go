package portfolio

import (
	"testing"

	"github.com/google/uuid"

	"openscum/internal/schedule"
)

func node(parent *uuid.UUID, program bool, lifecycle string, effort int32, plan float64, hasPlan bool, real float64) Node {
	return Node{ID: uuid.New(), ParentID: parent, Name: "n", Lifecycle: lifecycle, IsProgram: program, Effort: effort, Plan: plan, HasPlan: hasPlan, Real: real}
}

func TestAggregate(t *testing.T) {
	s := schedule.DefaultSettings()
	root := node(nil, true, "activo", 0, 0, false, 0) // no own work items
	a := node(&root.ID, false, "activo", 3, 100, true, 90)
	inner := node(&root.ID, true, "activo", 1, 50, true, 50)
	b := node(&inner.ID, false, "cerrado", 4, 100, true, 100) // closed counts (RF-15)
	gone := node(&root.ID, false, "archivado", 100, 100, true, 0)
	child := node(&gone.ID, false, "activo", 2, 0, false, 30) // under an archived one still counts
	tree := NewTree([]Node{root, a, inner, b, gone, child})

	// Each once: (3×90 + 1×50 + 4×100 + 2×30) / 10 = 78; plan (300 + 50 + 400) / 8 = 93.75.
	f := tree.Aggregate(root.ID, s)
	if f.Real != 78 || f.Plan != 93.8 || f.Level != schedule.Late {
		t.Fatalf("root = %+v", f)
	}
	// The nested program over its own subtree.
	if f := tree.Aggregate(inner.ID, s); f.Real != 90 || f.Plan != 90 {
		t.Fatalf("inner = %+v", f)
	}

	// A weight set by hand wins over the effort (RF-5).
	w := int32(0)
	b.ManualWeight = &w
	tree = NewTree([]Node{root, a, inner, b, gone, child})
	if f := tree.Aggregate(root.ID, s); f.Real != 63.3 { // (270 + 50 + 60) / 6
		t.Fatalf("with weight 0 = %+v", f)
	}

	card := tree.ProgramCard(root.ID, s)
	if card.Projects != 4 || len(card.Rows) != 2 || !card.Rows[1].IsProgram {
		t.Fatalf("card = %+v", card)
	}
}

func TestAggregateEdges(t *testing.T) {
	s := schedule.DefaultSettings()
	// All weigh 0: the simple mean (RF-17).
	root := node(nil, true, "activo", 0, 0, false, 0)
	x := node(&root.ID, false, "activo", 0, 40, true, 20)
	if f := NewTree([]Node{root, x}).Aggregate(root.ID, s); f.Real != 10 || f.Plan != 40 {
		t.Fatalf("weight 0 = %+v", f)
	}
	// Without plan: «Sin plazo» (RF-18); closed: no plan (RF-20).
	empty := node(nil, true, "activo", 0, 0, false, 0)
	if f := NewTree([]Node{empty}).Aggregate(empty.ID, s); f.HasPlan {
		t.Fatalf("empty = %+v", f)
	}
	closed := node(nil, true, "cerrado", 1, 50, true, 10)
	if f := NewTree([]Node{closed}).Aggregate(closed.ID, s); f.HasPlan || f.Real != 10 {
		t.Fatalf("closed = %+v", f)
	}

	// Loose projects: neither programs nor under one (RF-28).
	p := node(nil, true, "activo", 1, 0, false, 0)
	in := node(&p.ID, false, "activo", 1, 50, true, 10) // at risk? gap −40: late
	free := node(nil, false, "activo", 1, 50, true, 50)
	sub := node(&free.ID, false, "activo", 1, 0, false, 0)
	programs, loose := NewTree([]Node{p, in, free, sub}).Cards(s)
	if len(programs) != 1 || programs[0].BelowPlan != 1 || loose.Projects != 2 || loose.BelowPlan != 0 {
		t.Fatalf("programs = %+v, loose = %+v", programs, loose)
	}
}
