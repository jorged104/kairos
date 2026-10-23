package apitest

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

type objectiveDTO struct {
	ID         uuid.UUID `json:"id"`
	Status     string    `json:"status"`
	Version    int32     `json:"version"`
	Progress   int       `json:"progress"`
	KeyResults []struct {
		ID           uuid.UUID `json:"id"`
		Version      int32     `json:"version"`
		CurrentValue float64   `json:"current_value"`
		Progress     int       `json:"progress"`
		LinkedTotal  int       `json:"linked_total"`
		LinkedDone   int       `json:"linked_done"`
	} `json:"key_results"`
	Projects []struct {
		Identifier string `json:"identifier"`
	} `json:"projects"`
	Milestones []struct {
		Name    string `json:"name"`
		Project struct {
			Identifier string `json:"identifier"`
		} `json:"project"`
	} `json:"milestones"`
	Can struct {
		Manage bool `json:"manage"`
	} `json:"can"`
}

func (s *workSetup) objective(t *testing.T, c *Client, body map[string]any) objectiveDTO {
	t.Helper()
	if _, ok := body["period_start"]; !ok {
		body["period_start"], body["period_end"] = "2026-01-01", "2026-03-31"
	}
	if _, ok := body["title"]; !ok {
		body["title"] = "Objetivo"
	}
	var o objectiveDTO
	c.Post("/api/objectives", body).Expect(t, http.StatusCreated).JSON(t, &o)
	return o
}

func TestObjectiveScopesAndPermissions(t *testing.T) {
	s := newWorkSetup(t)
	project := s.workProject(t)

	s.lead.Post("/api/objectives", map[string]any{"title": "O", "scope": "organizacion", "period_start": "2026-01-01", "period_end": "2026-03-31"}).
		Expect(t, http.StatusForbidden, "forbidden")
	org := s.objective(t, s.admin, map[string]any{"scope": "organizacion"})
	team := s.objective(t, s.lead, map[string]any{"scope": "equipo", "scope_team_id": s.rep})
	proj := s.objective(t, s.lead, map[string]any{"scope": "proyecto", "scope_project_id": project, "owner_id": s.memberID})
	if org.Status != "borrador" || !team.Can.Manage || len(proj.Projects) != 1 {
		t.Fatalf("objectives = %+v / %+v / %+v", org, team, proj)
	}
	s.lead.Post("/api/objectives", map[string]any{"title": "O", "scope": "equipo", "scope_team_id": s.other, "period_start": "2026-01-01", "period_end": "2026-03-31"}).
		Expect(t, http.StatusForbidden, "forbidden")
	s.member.Post("/api/objectives", map[string]any{"title": "O", "scope": "equipo", "scope_team_id": s.rep, "period_start": "2026-01-01", "period_end": "2026-03-31"}).
		Expect(t, http.StatusForbidden, "forbidden")
	s.admin.Post("/api/objectives", map[string]any{"title": "O", "scope": "equipo", "period_start": "2026-01-01", "period_end": "2026-03-31"}).
		Expect(t, http.StatusBadRequest, "validation_failed")
	s.admin.Post("/api/objectives", map[string]any{"title": "O", "scope": "organizacion", "period_start": "2026-04-01", "period_end": "2026-03-31"}).
		Expect(t, http.StatusBadRequest, "invalid_date_range")

	// The owner (member) manages its objective; the lead manages those of its scope.
	var mine objectiveDTO
	s.member.Get("/api/objectives/"+proj.ID.String()).Expect(t, http.StatusOK).JSON(t, &mine)
	if !mine.Can.Manage {
		t.Fatal("owner cannot manage its objective")
	}
	s.member.Post("/api/objectives/"+org.ID.String()+"/key-results", map[string]any{"title": "KR", "start_value": 0, "target_value": 1, "calc_mode": "manual"}).
		Expect(t, http.StatusForbidden, "forbidden")
	s.lead.Post("/api/objectives/"+org.ID.String()+"/key-results", map[string]any{"title": "KR", "start_value": 0, "target_value": 1, "calc_mode": "manual"}).
		Expect(t, http.StatusForbidden, "forbidden")
	s.viewer.Get("/api/objectives?scope=equipo").Expect(t, http.StatusOK)

	var page struct {
		Total int `json:"total"`
	}
	s.viewer.Get("/api/objectives?scope=proyecto&from=2026-02-01&to=2026-02-28").Expect(t, http.StatusOK).JSON(t, &page)
	if page.Total != 1 {
		t.Fatalf("filtered objectives = %d (RF-131)", page.Total)
	}
	s.viewer.Get("/api/objectives?status=raro").Expect(t, http.StatusBadRequest, "invalid_filter")
}

func TestKeyResultsAndProgress(t *testing.T) {
	s := newWorkSetup(t)
	bir := s.workProject(t)
	bis := s.project(t, s.lead, map[string]any{"identifier": "BIS", "name": "Scrum"}).ID
	o := s.objective(t, s.lead, map[string]any{"scope": "equipo", "scope_team_id": s.bi})
	path := "/api/objectives/" + o.ID.String()

	// Activation needs key results (RF-100).
	s.lead.Post(path+"/activate", map[string]any{"version": o.Version}).Expect(t, http.StatusConflict, "key_result_required")
	s.lead.Post(path+"/key-results", map[string]any{"title": "KR", "start_value": 5, "target_value": 5, "calc_mode": "manual"}).
		Expect(t, http.StatusBadRequest, "key_result_target_equal")

	// Manual: reduce incidents from 40 to 10, now at 25 → 50 % (RF-103).
	s.lead.Post(path+"/key-results", map[string]any{"title": "Incidencias", "start_value": 40, "target_value": 10, "current_value": 25, "calc_mode": "manual"}).
		Expect(t, http.StatusCreated)
	var withDerived objectiveDTO
	s.lead.Post(path+"/key-results", map[string]any{"title": "Entregas", "start_value": 0, "target_value": 10, "calc_mode": "derivado"}).
		Expect(t, http.StatusCreated).JSON(t, &withDerived)
	if withDerived.KeyResults[0].Progress != 50 || withDerived.KeyResults[1].Progress != 0 || withDerived.Progress != 25 {
		t.Fatalf("progress = %+v (RF-103, RF-112, RF-113)", withDerived)
	}
	derived := withDerived.KeyResults[1]

	// Link a milestone of BIR and two work items of BIS to the derived KR.
	m1 := s.milestone(t, bir, map[string]any{"name": "M1"})
	wi1 := s.item(t, s.member, bis, map[string]any{})
	wi2 := s.item(t, s.member, bis, map[string]any{})
	krPath := path + "/key-results/" + derived.ID.String()
	s.lead.Post(krPath+"/milestones", map[string]any{"id": m1.ID}).Expect(t, http.StatusBadRequest, "milestone_not_in_objective") // RF-108
	s.lead.Post(path+"/milestones", map[string]any{"id": m1.ID}).Expect(t, http.StatusOK)
	s.lead.Post(krPath+"/milestones", map[string]any{"id": m1.ID}).Expect(t, http.StatusOK)
	s.lead.Post(krPath+"/work-items", map[string]any{"id": wi1.ID}).Expect(t, http.StatusOK)
	var linked objectiveDTO
	s.lead.Post(krPath+"/work-items", map[string]any{"id": wi2.ID}).Expect(t, http.StatusOK).JSON(t, &linked)
	if len(linked.Projects) != 2 || len(linked.Milestones) != 1 || linked.Milestones[0].Project.Identifier != "KAI" {
		t.Fatalf("links = %+v (RF-106, RF-115)", linked)
	}
	s.member.Post(krPath+"/work-items", map[string]any{"id": wi1.ID}).Expect(t, http.StatusForbidden, "forbidden") // RF-127

	// Closing a work item and completing the milestone move the derived KR (RF-110, RF-114).
	s.patch(s.member, wi1, map[string]any{"status_id": s.env.CatalogID("statuses", "Cerrado")}).Expect(t, http.StatusOK)
	s.lead.Post("/api/milestones/"+m1.ID.String()+"/complete", map[string]any{"version": m1.Version}).Expect(t, http.StatusOK)
	var now objectiveDTO
	s.viewer.Get(path).Expect(t, http.StatusOK).JSON(t, &now)
	kr := now.KeyResults[1]
	if kr.LinkedTotal != 3 || kr.LinkedDone != 2 || kr.Progress != 67 || kr.CurrentValue < 6.66 || kr.CurrentValue > 6.67 {
		t.Fatalf("derived KR = %+v", kr)
	}
	// Its current value cannot be set by hand (RF-111).
	s.lead.Do(http.MethodPatch, krPath, map[string]any{"version": kr.Version, "title": "Entregas", "start_value": 0, "target_value": 10,
		"current_value": 9, "calc_mode": "derivado"}).Expect(t, http.StatusBadRequest, "key_result_derived")

	// Unlinking the milestone from the objective drops it from the KR (RF-109).
	var after objectiveDTO
	s.lead.Do(http.MethodDelete, path+"/milestones/"+m1.ID.String(), nil).Expect(t, http.StatusOK).JSON(t, &after)
	if after.KeyResults[1].LinkedTotal != 2 || len(after.Projects) != 2 {
		t.Fatalf("after unlink = %+v", after.KeyResults[1])
	}

	// Lifecycle; closed objectives are read-only.
	var active objectiveDTO
	s.lead.Post(path+"/activate", map[string]any{"version": after.Version}).Expect(t, http.StatusOK).JSON(t, &active)
	var closed objectiveDTO
	s.lead.Post(path+"/close", map[string]any{"version": active.Version}).Expect(t, http.StatusOK).JSON(t, &closed)
	s.lead.Post(path+"/key-results", map[string]any{"title": "X", "start_value": 0, "target_value": 1, "calc_mode": "manual"}).
		Expect(t, http.StatusConflict, "objective_closed")
}
