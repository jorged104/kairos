package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type portfolioRowDTO struct {
	ID        uuid.UUID  `json:"id"`
	Name      string     `json:"name"`
	IsProgram bool       `json:"is_program"`
	Weight    float64    `json:"weight"`
	Figures   figuresDTO `json:"figures"`
}

type portfolioCardDTO struct {
	Program   *portfolioRowDTO  `json:"program"`
	Projects  int               `json:"projects"`
	BelowPlan int               `json:"below_plan"`
	Rows      []portfolioRowDTO `json:"rows"`
}

type portfolioDTO struct {
	Today    string             `json:"today"`
	Programs []portfolioCardDTO `json:"programs"`
	Loose    portfolioCardDTO   `json:"loose"`
}

type portfolioProjectDTO struct {
	Version         int32  `json:"version"`
	IsProgram       bool   `json:"is_program"`
	PortfolioWeight *int32 `json:"portfolio_weight"`
	Effort          int32  `json:"effort"`
	Can             struct {
		ManagePortfolio bool `json:"manage_portfolio"`
	} `json:"can"`
}

func (s *workSetup) portfolioOf(t *testing.T, id uuid.UUID) portfolioProjectDTO {
	t.Helper()
	var p portfolioProjectDTO
	s.admin.Get("/api/projects/"+id.String()).Expect(t, http.StatusOK).JSON(t, &p)
	return p
}

func (s *workSetup) setPortfolio(c *Client, id uuid.UUID, version int32, program bool, weight any) *Response {
	return c.Do(http.MethodPut, "/api/projects/"+id.String()+"/portfolio", map[string]any{"version": version, "is_program": program, "weight": weight})
}

func TestProgramMarking(t *testing.T) {
	s := newWorkSetup(t)
	p := s.project(t, s.lead, map[string]any{"identifier": "PRG", "name": "Programa"})
	s.item(t, s.member, p.ID, map[string]any{"weight": 3})
	s.item(t, s.member, p.ID, map[string]any{"weight": 2})
	cur := s.portfolioOf(t, p.ID)
	if cur.IsProgram || cur.PortfolioWeight != nil || cur.Effort != 5 || !cur.Can.ManagePortfolio { // RF-8
		t.Fatalf("before = %+v", cur)
	}
	// Only the administrator (RF-3, RF-7).
	s.setPortfolio(s.lead, p.ID, cur.Version, true, nil).Expect(t, http.StatusForbidden, "forbidden")
	s.setPortfolio(s.admin, p.ID, cur.Version, true, 100001).Expect(t, http.StatusBadRequest, "validation_failed") // RF-5
	var after portfolioProjectDTO
	s.setPortfolio(s.admin, p.ID, cur.Version, true, 7).Expect(t, http.StatusOK).JSON(t, &after)
	if !after.IsProgram || after.PortfolioWeight == nil || *after.PortfolioWeight != 7 {
		t.Fatalf("after = %+v", after)
	}
	s.setPortfolio(s.admin, p.ID, cur.Version, false, nil).Expect(t, http.StatusConflict, "version_conflict")
	s.setPortfolio(s.admin, p.ID, after.Version, true, nil).Expect(t, http.StatusOK).JSON(t, &after) // RF-6
	if after.PortfolioWeight != nil {
		t.Fatalf("weight not cleared = %+v", after)
	}
	acts := s.env.AuditActions(p.ID.String())
	if n := len(acts); n < 3 || acts[n-3] != "project.program_set" || acts[n-2] != "project.portfolio_weight_changed" || acts[n-1] != "project.portfolio_weight_changed" { // RF-9, RF-10
		t.Fatalf("audit = %v", acts)
	}
	var lead portfolioProjectDTO
	s.lead.Get("/api/projects/"+p.ID.String()).Expect(t, http.StatusOK).JSON(t, &lead)
	if lead.Can.ManagePortfolio {
		t.Fatal("a lead can not manage the portfolio")
	}
}

func TestPortfolio(t *testing.T) {
	s := newWorkSetup(t)
	// Root program R: A (effort 3, 100 % planned, 60 % done), inner program I with
	// B (effort 1, plan 100, real 100). Loose project L.
	root := s.project(t, s.lead, map[string]any{"identifier": "RAIZ", "name": "Raíz"})
	a := s.project(t, s.lead, map[string]any{"identifier": "AAA", "name": "A", "parent_id": root.ID})
	inner := s.project(t, s.lead, map[string]any{"identifier": "INT", "name": "Interno", "parent_id": root.ID})
	b := s.project(t, s.lead, map[string]any{"identifier": "BBB", "name": "B", "parent_id": inner.ID})
	loose := s.project(t, s.lead, map[string]any{"identifier": "SUE", "name": "Suelto"})
	s.item(t, s.member, a.ID, map[string]any{"weight": 3, "due_date": "2026-03-01", "done_ratio": 60})
	s.item(t, s.member, b.ID, map[string]any{"weight": 1, "due_date": "2026-03-01", "done_ratio": 100})
	s.item(t, s.member, loose.ID, map[string]any{"weight": 1, "due_date": "2026-03-01", "done_ratio": 100})
	for _, id := range []uuid.UUID{root.ID, inner.ID} {
		s.setPortfolio(s.admin, id, s.portfolioOf(t, id).Version, true, nil).Expect(t, http.StatusOK)
	}

	var pf portfolioDTO
	s.viewer.Get("/api/portfolio?today=2026-03-06").Expect(t, http.StatusOK).JSON(t, &pf)
	if len(pf.Programs) != 2 || pf.Programs[0].Program.Name != "Interno" || pf.Programs[1].Program.Name != "Raíz" { // RF-22, by name
		t.Fatalf("programs = %+v", pf.Programs)
	}
	r := pf.Programs[1]
	// (3 × 60 + 1 × 100) / 4 = 70 against 100: late, and A is below plan (RF-11, RF-24).
	if f := r.Program.Figures; f.Real != 70 || f.Plan != 100 || f.Level != "late" || r.Projects != 3 || r.BelowPlan != 1 {
		t.Fatalf("root = %+v", r)
	}
	if len(r.Rows) != 2 || r.Rows[0].Name != "A" || r.Rows[0].Weight != 3 || !r.Rows[1].IsProgram || r.Rows[1].Figures.Real != 100 { // RF-25 to RF-27
		t.Fatalf("rows = %+v", r.Rows)
	}
	if len(pf.Loose.Rows) != 1 || pf.Loose.Rows[0].Name != "Suelto" || pf.Loose.Program != nil { // RF-28
		t.Fatalf("loose = %+v", pf.Loose)
	}

	// Archived: it no longer counts (RF-16).
	s.lead.Post("/api/projects/"+a.ID.String()+"/archive", map[string]any{"version": s.getProject(t, a.ID).Version}).Expect(t, http.StatusOK)
	s.viewer.Get("/api/portfolio?today=2026-03-06").Expect(t, http.StatusOK).JSON(t, &pf)
	if r := pf.Programs[1]; r.Program.Figures.Real != 100 || len(r.Rows) != 1 || r.Projects != 2 {
		t.Fatalf("after archiving = %+v", r)
	}

	// The overview of a program carries its card (RF-39, RF-40).
	var ov struct {
		Program *portfolioCardDTO `json:"program"`
	}
	s.viewer.Get("/api/projects/"+root.ID.String()+"/overview?today=2026-03-06").Expect(t, http.StatusOK).JSON(t, &ov)
	if ov.Program == nil || ov.Program.Program.Figures.Real != 100 || len(ov.Program.Rows) != 1 {
		t.Fatalf("overview program = %+v", ov.Program)
	}
	ov.Program = nil
	s.viewer.Get("/api/projects/"+loose.ID.String()+"/overview?today=2026-03-06").Expect(t, http.StatusOK).JSON(t, &ov)
	if ov.Program != nil {
		t.Fatal("a project that is not a program has no card")
	}
	s.viewer.Get("/api/portfolio?today=mañana").Expect(t, http.StatusBadRequest, "invalid_filter")
}
