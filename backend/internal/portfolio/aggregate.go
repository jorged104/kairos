// Package portfolio adds up the plan of the projects of each program and
// lays out the portfolio (spec 010).
package portfolio

import (
	"math"

	"github.com/google/uuid"

	"openscum/internal/schedule"
)

const (
	lifecycleActive   = "activo"
	lifecycleArchived = "archivado"
)

// Node is a project of the portfolio with its own plan and real progress.
type Node struct {
	ID           uuid.UUID
	ParentID     *uuid.UUID
	Identifier   string
	Name         string
	Lifecycle    string
	IsProgram    bool
	ManualWeight *int32
	Effort       int32
	Plan         float64
	HasPlan      bool
	Real         float64
}

// Weight in a program: the one set by hand, else its effort (Conceptos).
func (n Node) Weight() float64 {
	if n.ManualWeight != nil {
		return float64(*n.ManualWeight)
	}
	return float64(n.Effort)
}

// Own figures of a project; a closed one shows no plan (spec 009, RF-34).
func (n Node) Own(s schedule.Settings) schedule.Figures {
	return schedule.Make(n.Plan, n.HasPlan && n.Lifecycle == lifecycleActive, n.Real, s)
}

// Tree indexes the nodes by parent.
type Tree struct {
	Nodes    map[uuid.UUID]Node
	Children map[uuid.UUID][]uuid.UUID
	Order    []uuid.UUID // by name
	// The work items of each project, when loaded from the database.
	Items map[uuid.UUID][]schedule.Item
}

func NewTree(nodes []Node) Tree {
	t := Tree{Nodes: make(map[uuid.UUID]Node, len(nodes)), Children: map[uuid.UUID][]uuid.UUID{}}
	for _, n := range nodes {
		t.Nodes[n.ID] = n
		t.Order = append(t.Order, n.ID)
		if n.ParentID != nil {
			t.Children[*n.ParentID] = append(t.Children[*n.ParentID], n.ID)
		}
	}
	return t
}

// Members are the program and every descendant not archived, each once
// (RF-13, RF-14, RF-16); archived ones are walked through.
func (t Tree) Members(id uuid.UUID) []Node {
	var out []Node
	seen := map[uuid.UUID]bool{}
	var walk func(uuid.UUID)
	walk = func(id uuid.UUID) {
		if seen[id] {
			return
		}
		seen[id] = true
		n := t.Nodes[id]
		if n.Lifecycle != lifecycleArchived {
			out = append(out, n)
		}
		for _, c := range t.Children[id] {
			walk(c)
		}
	}
	walk(id)
	return out
}

// Aggregate is the plan and the real progress of a program over its members,
// weighted by their weight; the simple mean when all weigh 0 (RF-11, RF-12,
// RF-17 to RF-20).
func (t Tree) Aggregate(id uuid.UUID, s schedule.Settings) schedule.Figures {
	f := Combine(t.Members(id), s)
	if t.Nodes[id].Lifecycle != lifecycleActive { // RF-20
		f = schedule.Figures{Real: f.Real}
	}
	return f
}

// Combine is the mean of the plan and the real progress of some projects,
// weighted by their weight (spec 011, RF-13).
func Combine(members []Node, s schedule.Settings) schedule.Figures {
	var w, wr, wp, pw, plain, plans float64
	var planned int
	for _, m := range members {
		w += m.Weight()
		wr += m.Weight() * m.Real
		plain += m.Real
		if m.HasPlan {
			pw += m.Weight()
			wp += m.Weight() * m.Plan
			plans += m.Plan
			planned++
		}
	}
	real := 0.0
	switch {
	case w > 0:
		real = wr / w
	case len(members) > 0:
		real = plain / float64(len(members))
	}
	plan, hasPlan := 0.0, planned > 0
	switch {
	case pw > 0:
		plan = wp / pw
	case planned > 0:
		plan = plans / float64(planned)
	}
	return schedule.Make(clamp(plan), hasPlan, clamp(real), s)
}

func clamp(v float64) float64 { return math.Min(100, math.Max(0, v)) }

// Row is a direct child of a program in its card (RF-25 to RF-27).
type Row struct {
	ID         uuid.UUID        `json:"id"`
	Identifier string           `json:"identifier"`
	Name       string           `json:"name"`
	Lifecycle  string           `json:"lifecycle"`
	IsProgram  bool             `json:"is_program"`
	Weight     float64          `json:"weight"`
	Figures    schedule.Figures `json:"figures"`
}

// Card is a program with its direct children, or the projects without a
// program (RF-22 to RF-28).
type Card struct {
	Program   *Row  `json:"program"` // null: «Sin programa»
	Projects  int   `json:"projects"`
	BelowPlan int   `json:"below_plan"`
	Rows      []Row `json:"rows"`
}

func (t Tree) row(n Node, s schedule.Settings) Row {
	r := Row{ID: n.ID, Identifier: n.Identifier, Name: n.Name, Lifecycle: n.Lifecycle, IsProgram: n.IsProgram, Weight: n.Weight()}
	if n.IsProgram {
		r.Figures = t.Aggregate(n.ID, s)
	} else {
		r.Figures = n.Own(s)
	}
	return r
}

func below(f schedule.Figures) bool { return f.Level == schedule.AtRisk || f.Level == schedule.Late }

// ProgramCard is the card of one program.
func (t Tree) ProgramCard(id uuid.UUID, s schedule.Settings) Card {
	n := t.Nodes[id]
	head := t.row(n, s)
	c := Card{Program: &head, Rows: []Row{}}
	for _, m := range t.Members(id) {
		if m.ID == id {
			continue
		}
		c.Projects++
		if below(m.Own(s)) { // RF-24
			c.BelowPlan++
		}
	}
	for _, ch := range t.Children[id] {
		if cn := t.Nodes[ch]; cn.Lifecycle != lifecycleArchived {
			c.Rows = append(c.Rows, t.row(cn, s))
		}
	}
	return c
}

// Cards are one per program not archived, by name, and «Sin programa» with
// the projects that are neither programs nor under one (RF-22, RF-28).
func (t Tree) Cards(s schedule.Settings) (programs []Card, loose Card) {
	programs = []Card{}
	loose = Card{Rows: []Row{}}
	for _, id := range t.Order {
		n := t.Nodes[id]
		if n.Lifecycle == lifecycleArchived {
			continue
		}
		if n.IsProgram {
			programs = append(programs, t.ProgramCard(id, s))
			continue
		}
		if !t.underProgram(n) {
			loose.Projects++
			r := t.row(n, s)
			if below(r.Figures) {
				loose.BelowPlan++
			}
			loose.Rows = append(loose.Rows, r)
		}
	}
	return programs, loose
}

// underProgram: some ancestor not archived is a program.
func (t Tree) underProgram(n Node) bool {
	for p := n.ParentID; p != nil; {
		a, ok := t.Nodes[*p]
		if !ok {
			return false
		}
		if a.IsProgram && a.Lifecycle != lifecycleArchived {
			return true
		}
		p = a.ParentID
	}
	return false
}
