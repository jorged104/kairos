// Package pages saves the pages of projects, milestones and work items
// (spec 005) and receives their files. It knows who may change each owner.
package pages

import (
	"context"

	"github.com/google/uuid"

	"openscum/internal/apperr"
	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/files"
	"openscum/internal/milestones"
	"openscum/internal/projects"
	"openscum/internal/workitems"
)

// Owner kinds, as page_files stores them.
const (
	KindProject   = "project"
	KindMilestone = "milestone"
	KindWorkItem  = "work_item"
)

// owner is a page's owner with what saving its page needs.
type owner struct {
	files.Owner
	project     sqlc.Project
	pageVersion int32
	description []byte
}

// writable loads the owner p wants to change, locked when lock is set, and
// checks that p may change it with the permission of its own edits: work
// items with CanWriteWorkItems, milestones and projects with CanManageProject.
func writable(ctx context.Context, q *sqlc.Queries, p *authz.Principal, kind string, id uuid.UUID, lock bool) (owner, error) {
	var o owner
	switch kind {
	case KindWorkItem:
		w, project, err := workitems.Load(ctx, q, p, id, lock)
		if err != nil {
			return o, err
		}
		if !p.CanWriteWorkItems(projects.Ref(project)) {
			return o, apperr.ErrForbidden
		}
		o = owner{project: project, pageVersion: w.PageVersion, description: w.Description}
	case KindMilestone:
		m, project, err := milestones.Load(ctx, q, p, id, lock)
		if err != nil {
			return o, err
		}
		if !p.CanManageProject(projects.Ref(project)) {
			return o, apperr.ErrForbidden
		}
		o = owner{project: project, pageVersion: m.PageVersion, description: m.Description}
	case KindProject:
		project, err := projects.Get(ctx, q, p, id, lock)
		if err != nil {
			return o, err
		}
		if !p.CanManageProject(projects.Ref(project)) {
			return o, apperr.ErrForbidden
		}
		o = owner{project: project, pageVersion: project.PageVersion, description: project.Description}
	default:
		return o, apperr.ErrNotFound
	}
	o.Owner = files.Owner{Kind: kind, ID: id, ProjectID: o.project.ID}
	return o, projects.Writable(o.project)
}
