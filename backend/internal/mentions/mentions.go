// Package mentions resolves the mentions of the pages of spec 005 when they
// are read, and searches what can be mentioned (RF-25 to RF-30). Mentions
// only link: nobody is notified (RF-30).
package mentions

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/richtext"
)

// States of a mention for whoever reads the page.
const (
	StateOK       = "ok"
	StateNoAccess = "no_access" // never carries the label (RF-28)
	StateDeleted  = "deleted"   // RF-29
)

// Ref is how a mention reads now: the current name of the user, or the
// current key and title of the element (RF-27).
type Ref struct {
	Label string `json:"label,omitempty"`
	State string `json:"state"`
}

// Refs resolves the mentions of doc for p, keyed by the mentioned id. It
// makes one query per kind present.
func Refs(ctx context.Context, q *sqlc.Queries, p *authz.Principal, doc json.RawMessage) (map[string]Ref, error) {
	out := map[string]Ref{}
	ids := map[string][]uuid.UUID{}
	for _, m := range richtext.Mentions(doc) {
		if _, seen := out[m.ID.String()]; !seen {
			out[m.ID.String()] = Ref{State: StateDeleted} // until found
			ids[m.Kind] = append(ids[m.Kind], m.ID)
		}
	}
	templates := p.Has(authz.TemplatesUse)
	// A template for who may use it; any other project only if seen (spec 023, RF-28).
	visible := func(isTemplate bool, pr authz.ProjectRef, label string) Ref {
		if isTemplate && !templates || !isTemplate && !p.SeesProject(pr) {
			return Ref{State: StateNoAccess}
		}
		return Ref{Label: label, State: StateOK}
	}
	if len(ids["user"]) > 0 {
		rows, err := q.MentionedUsers(ctx, ids["user"])
		if err != nil {
			return nil, fmt.Errorf("usuarios mencionados: %w", err)
		}
		for _, u := range rows {
			if u.Status != "offboarded" {
				out[u.ID.String()] = Ref{Label: u.Name, State: StateOK}
			}
		}
	}
	if len(ids["work_item"]) > 0 {
		rows, err := q.MentionedWorkItems(ctx, ids["work_item"])
		if err != nil {
			return nil, fmt.Errorf("work items mencionados: %w", err)
		}
		for _, w := range rows {
			out[w.ID.String()] = visible(w.IsTemplate, authz.ProjectRef{ID: w.ProjectID, TeamID: w.TeamID, OwnerID: w.OwnerID}, key(w.Identifier, w.SequenceNumber)+" "+w.Title)
		}
	}
	if len(ids["milestone"]) > 0 {
		rows, err := q.MentionedMilestones(ctx, ids["milestone"])
		if err != nil {
			return nil, fmt.Errorf("hitos mencionados: %w", err)
		}
		for _, m := range rows {
			out[m.ID.String()] = visible(m.IsTemplate, authz.ProjectRef{ID: m.ProjectID, TeamID: m.TeamID, OwnerID: m.OwnerID}, m.Name)
		}
	}
	if len(ids["meeting"]) > 0 {
		rows, err := q.MentionedMeetings(ctx, ids["meeting"])
		if err != nil {
			return nil, fmt.Errorf("reuniones mencionadas: %w", err)
		}
		for _, m := range rows {
			out[m.ID.String()] = visible(m.IsTemplate, authz.ProjectRef{ID: m.ProjectID, TeamID: m.TeamID, OwnerID: m.OwnerID}, m.Title)
		}
	}
	if len(ids["project"]) > 0 {
		rows, err := q.MentionedProjects(ctx, ids["project"])
		if err != nil {
			return nil, fmt.Errorf("proyectos mencionados: %w", err)
		}
		for _, pr := range rows {
			out[pr.ID.String()] = visible(pr.IsTemplate, authz.ProjectRef{ID: pr.ID, TeamID: pr.TeamID, OwnerID: pr.OwnerID}, pr.Identifier+" "+pr.Name)
		}
	}
	return out, nil
}

func key(identifier string, n int32) string { return identifier + "-" + strconv.Itoa(int(n)) }

// Result is something to mention, as the # search lists it.
type Result struct {
	Kind  string    `json:"kind"`
	ID    uuid.UUID `json:"id"`
	Label string    `json:"label"`
}

// Results holds up to 10 matches of each kind.
type Results struct {
	WorkItems  []Result `json:"work_items"`
	Milestones []Result `json:"milestones"`
	Projects   []Result `json:"projects"`
	Meetings   []Result `json:"meetings"` // spec 018, RF-18
}

var likeEscaper = strings.NewReplacer(`\`, `\`, `%`, `\%`, `_`, `\_`)

// Search finds work items by key or title, milestones and projects that p can
// read, for the # search (RF-26). The scope is applied in the query.
func Search(ctx context.Context, q *sqlc.Queries, p *authz.Principal, text string) (Results, error) {
	out := Results{WorkItems: []Result{}, Milestones: []Result{}, Projects: []Result{}, Meetings: []Result{}}
	text = strings.TrimSpace(text)
	if text == "" {
		return out, nil
	}
	escaped := likeEscaper.Replace(text)
	prefix, contains := escaped+"%", "%"+escaped+"%"
	templates, all, seen, me, teams := p.Has(authz.TemplatesUse), p.SeesAll(), p.VisibleProjectIDs(), p.UserID, p.VisibleTeamIDs()

	items, err := q.SearchMentionWorkItems(ctx, sqlc.SearchMentionWorkItemsParams{Templates: templates, Prefix: prefix, Contains: contains, SeeAll: all, Seen: seen, Me: me, SeenTeams: teams})
	if err != nil {
		return out, fmt.Errorf("buscar work items: %w", err)
	}
	for _, w := range items {
		out.WorkItems = append(out.WorkItems, Result{Kind: "work_item", ID: w.ID, Label: key(w.Identifier, w.SequenceNumber) + " " + w.Title})
	}
	ms, err := q.SearchMentionMilestones(ctx, sqlc.SearchMentionMilestonesParams{Templates: templates, Contains: contains, SeeAll: all, Seen: seen, Me: me, SeenTeams: teams})
	if err != nil {
		return out, fmt.Errorf("buscar hitos: %w", err)
	}
	for _, m := range ms {
		out.Milestones = append(out.Milestones, Result{Kind: "milestone", ID: m.ID, Label: m.Name + " · " + m.Identifier})
	}
	ps, err := q.SearchMentionProjects(ctx, sqlc.SearchMentionProjectsParams{Templates: templates, Prefix: prefix, Contains: contains, SeeAll: all, Seen: seen, Me: me, SeenTeams: teams})
	if err != nil {
		return out, fmt.Errorf("buscar proyectos: %w", err)
	}
	for _, pr := range ps {
		out.Projects = append(out.Projects, Result{Kind: "project", ID: pr.ID, Label: pr.Identifier + " " + pr.Name})
	}
	mt, err := q.SearchMentionMeetings(ctx, sqlc.SearchMentionMeetingsParams{Templates: templates, Contains: contains, SeeAll: all, Seen: seen, Me: me, SeenTeams: teams})
	if err != nil {
		return out, fmt.Errorf("buscar reuniones: %w", err)
	}
	for _, m := range mt {
		out.Meetings = append(out.Meetings, Result{Kind: "meeting", ID: m.ID, Label: m.Title + " · " + m.Identifier})
	}
	return out, nil
}
