package workitems

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/catalog"
	"openscum/internal/customfields"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/events"
	"openscum/internal/projects"
	"openscum/internal/richtext"
	"openscum/internal/workcore"
)

type Service struct {
	store *db.Store
}

func NewService(store *db.Store) *Service { return &Service{store: store} }

// Key is the readable identifier of a work item, e.g. KAI-42 (RF-62).
func Key(identifier string, n int32) string { return identifier + "-" + strconv.Itoa(int(n)) }

// Load returns a work item and its project if the caller can see them.
func Load(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID, forUpdate bool) (sqlc.WorkItem, sqlc.Project, error) {
	get := q.GetWorkItem
	if forUpdate {
		get = q.GetWorkItemForUpdate
	}
	row, err := get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return row, sqlc.Project{}, apperr.ErrNotFound
	}
	if err != nil {
		return row, sqlc.Project{}, fmt.Errorf("leer work item: %w", err)
	}
	project, err := projects.Get(ctx, q, p, row.ProjectID, false)
	return row, project, err
}

func (s *Service) Get(ctx context.Context, p *authz.Principal, id uuid.UUID) (WorkItem, error) {
	row, project, err := Load(ctx, s.store.Queries, p, id, false)
	if err != nil {
		return WorkItem{}, err
	}
	return view(ctx, s.store.Queries, p, row, project)
}

// GetByKey opens a work item by its readable key (RF-133).
func (s *Service) GetByKey(ctx context.Context, p *authz.Principal, key string) (WorkItem, error) {
	identifier, num, ok := strings.Cut(strings.ToUpper(strings.TrimSpace(key)), "-")
	n, err := strconv.Atoi(num)
	if !ok || err != nil || n < 1 {
		return WorkItem{}, apperr.ErrNotFound
	}
	row, err := s.store.GetWorkItemByKey(ctx, sqlc.GetWorkItemByKeyParams{Identifier: identifier, SequenceNumber: int32(n)})
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkItem{}, apperr.ErrNotFound
	}
	if err != nil {
		return WorkItem{}, fmt.Errorf("leer work item: %w", err)
	}
	return s.Get(ctx, p, row.ID)
}

func checkNumbers(est, rem *float64, ratio int16) error {
	for name, v := range map[string]*float64{"estimated_hours": est, "remaining_hours": rem} {
		if v != nil && (*v < 0 || *v > 10000) {
			return errOutOfRange.WithDetail("field", name)
		}
	}
	if ratio < 0 || ratio > 100 {
		return errOutOfRange.WithDetail("field", "done_ratio")
	}
	return nil
}

func checkType(ctx context.Context, q *sqlc.Queries, project, typeID uuid.UUID) error {
	t, err := q.GetWorkItemType(ctx, typeID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && t.RetiredAt != nil) {
		return errTypeNotEnabled
	}
	if err != nil {
		return fmt.Errorf("leer tipo: %w", err)
	}
	enabled, err := q.ProjectTypeEnabled(ctx, sqlc.ProjectTypeEnabledParams{ProjectID: project, TypeID: typeID})
	if err != nil {
		return fmt.Errorf("comprobar tipo: %w", err)
	}
	if !enabled {
		return errTypeNotEnabled
	}
	return nil
}

func checkPriority(ctx context.Context, q *sqlc.Queries, id uuid.UUID) error {
	pr, err := q.GetPriority(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && pr.RetiredAt != nil) {
		return apperr.ErrValidation.WithDetail("field", "priority_id")
	}
	if err != nil {
		return fmt.Errorf("leer prioridad: %w", err)
	}
	return nil
}

// checkParent validates a (new) parent for a work item of project whose
// subtree is height levels deep and returns the new depth. The caller holds
// the project tree lock.
func checkParent(ctx context.Context, q *sqlc.Queries, project uuid.UUID, id uuid.UUID, parentID *uuid.UUID, height int32) (int16, error) {
	if parentID == nil {
		return 0, nil
	}
	parent, err := q.GetWorkItem(ctx, *parentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, apperr.ErrValidation.WithDetail("field", "parent_id")
	}
	if err != nil {
		return 0, fmt.Errorf("leer padre: %w", err)
	}
	if parent.ProjectID != project {
		return 0, errCrossProject.WithDetail("field", "parent_id")
	}
	if id != uuid.Nil {
		inside, err := q.WorkItemIsDescendant(ctx, sqlc.WorkItemIsDescendantParams{WorkItemID: id, Candidate: *parentID})
		if err != nil {
			return 0, fmt.Errorf("comprobar ciclo: %w", err)
		}
		if inside {
			return 0, errCycle
		}
	}
	depth := parent.Depth + 1
	if int32(depth)+height > maxDepth {
		return 0, errTooDeep
	}
	return depth, nil
}

func applyValues(ctx context.Context, q *sqlc.Queries, project, id, typeID uuid.UUID, values customfields.Values) error {
	defs, err := customfields.Load(ctx, q)
	if err != nil {
		return err
	}
	enabled, err := customfields.EnabledFields(ctx, q, project)
	if err != nil {
		return err
	}
	_, err = customfields.Apply(ctx, defs, customfields.WorkItemValues(q, id),
		customfields.Scope{Entity: customfields.EntityWorkItem, Enabled: enabled, TypeID: typeID}, values, true)
	return err
}

// Create adds a work item to a project (RF-61 to RF-72).
func (s *Service) Create(ctx context.Context, p *authz.Principal, projectID uuid.UUID, req createRequest) (WorkItem, error) {
	if err := workcore.CheckRange(req.StartDate, req.DueDate, errDateRange); err != nil {
		return WorkItem{}, err
	}
	if err := checkNumbers(req.EstimatedHours, req.RemainingHours, req.DoneRatio); err != nil {
		return WorkItem{}, err
	}
	var out WorkItem
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		project, err := projects.Get(ctx, q, p, projectID, false)
		if err != nil {
			return err
		}
		if !p.CanWriteWorkItems(projects.Ref(project)) {
			return apperr.ErrForbidden
		}
		if err := projects.Writable(project); err != nil {
			return err
		}
		if err := checkType(ctx, q, projectID, req.TypeID); err != nil {
			return err
		}
		status, err := initialStatus(ctx, q, req.StatusID)
		if err != nil {
			return err
		}
		priority := req.PriorityID
		if priority == nil {
			def, err := q.GetDefaultPriority(ctx)
			if err != nil {
				return fmt.Errorf("prioridad por defecto: %w", err)
			}
			priority = &def.ID
		} else if err := checkPriority(ctx, q, *priority); err != nil {
			return err
		}
		if req.AssigneeID != nil {
			if err := workcore.CheckWorker(ctx, q, *req.AssigneeID, errInvalidAssignee); err != nil {
				return err
			}
		}
		depth := int16(0)
		if req.ParentID != nil {
			if err := q.LockWorkItemTree(ctx, projectID); err != nil {
				return fmt.Errorf("bloquear árbol: %w", err)
			}
			if depth, err = checkParent(ctx, q, projectID, uuid.Nil, req.ParentID, 0); err != nil {
				return err
			}
		}
		desc, text, err := richtext.Clean(req.Description)
		if err != nil {
			return err
		}
		n, err := q.NextWorkItemNumber(ctx, projectID)
		if err != nil {
			return fmt.Errorf("numerar work item: %w", err)
		}
		row, err := q.CreateWorkItem(ctx, sqlc.CreateWorkItemParams{
			ProjectID: projectID, TypeID: req.TypeID, StatusID: status, PriorityID: *priority, ParentID: req.ParentID,
			SequenceNumber: n, Title: req.Title, Description: desc, DescriptionText: text, AuthorID: p.UserID,
			AssigneeID: req.AssigneeID, StartDate: workcore.ToTime(req.StartDate), DueDate: workcore.ToTime(req.DueDate),
			EstimatedHours: req.EstimatedHours, RemainingHours: req.RemainingHours, DoneRatio: req.DoneRatio,
			Position: req.Position, Depth: depth,
		})
		if err != nil {
			return fmt.Errorf("crear work item: %w", err)
		}
		if err := applyValues(ctx, q, projectID, row.ID, req.TypeID, req.CustomValues); err != nil {
			return err
		}
		if err := q.RefreshWorkItemUp(ctx, row.ID); err != nil {
			return fmt.Errorf("recalcular derivados: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "work_item.created", TargetType: "work_item",
			TargetID: row.ID.String(), After: auditView(row, Key(project.Identifier, n))}); err != nil {
			return err
		}
		if row, err = q.GetWorkItem(ctx, row.ID); err != nil {
			return fmt.Errorf("releer work item: %w", err)
		}
		out, err = view(ctx, q, p, row, project)
		return err
	})
	return out, err
}

// initialStatus is the default status or an active, open one (RF-64).
func initialStatus(ctx context.Context, q *sqlc.Queries, id *uuid.UUID) (uuid.UUID, error) {
	if id == nil {
		def, err := q.GetDefaultStatus(ctx)
		if err != nil {
			return uuid.Nil, fmt.Errorf("estado por defecto: %w", err)
		}
		return def.ID, nil
	}
	st, err := q.GetStatus(ctx, *id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (st.RetiredAt != nil || st.IsClosed)) {
		return uuid.Nil, apperr.ErrValidation.WithDetail("field", "status_id")
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("leer estado: %w", err)
	}
	return st.ID, nil
}

// Update replaces the editable fields of a work item, including its status
// (workflow, blocking, events) and parent (cycles, depth, derived values).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req updateRequest) (WorkItem, error) {
	if err := workcore.CheckRange(req.StartDate, req.DueDate, errDateRange); err != nil {
		return WorkItem{}, err
	}
	if err := checkNumbers(req.EstimatedHours, req.RemainingHours, req.DoneRatio); err != nil {
		return WorkItem{}, err
	}
	var out WorkItem
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, project, err := Load(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if !p.CanWriteWorkItems(projects.Ref(project)) {
			return apperr.ErrForbidden
		}
		if err := projects.Writable(project); err != nil {
			return err
		}
		if req.Version != row.Version {
			return apperr.VersionConflict(row.Version)
		}
		if req.TypeID != row.TypeID {
			if err := checkType(ctx, q, project.ID, req.TypeID); err != nil {
				return err
			}
		}
		if req.PriorityID != row.PriorityID {
			if err := checkPriority(ctx, q, req.PriorityID); err != nil {
				return err
			}
		}
		if req.AssigneeID != nil && !sameID(req.AssigneeID, row.AssigneeID) {
			if err := workcore.CheckWorker(ctx, q, *req.AssigneeID, errInvalidAssignee); err != nil {
				return err
			}
		}

		key := Key(project.Identifier, row.SequenceNumber)
		change, err := statusChange(ctx, q, p, row, req.StatusID, key)
		if err != nil {
			return err
		}
		ratio, remaining, closedAt := req.DoneRatio, req.RemainingHours, row.ClosedAt
		switch change {
		case closing:
			// Closing finishes the work item (RF-84).
			zero, now := 0.0, time.Now().UTC()
			ratio, remaining, closedAt = 100, &zero, &now
		case reopening:
			closedAt = nil
		}

		depth := row.Depth
		parentChanged := !sameID(req.ParentID, row.ParentID)
		if parentChanged {
			if err := q.LockWorkItemTree(ctx, project.ID); err != nil {
				return fmt.Errorf("bloquear árbol: %w", err)
			}
			height, err := q.WorkItemSubtreeHeight(ctx, id)
			if err != nil {
				return fmt.Errorf("altura del subárbol: %w", err)
			}
			if depth, err = checkParent(ctx, q, project.ID, id, req.ParentID, height); err != nil {
				return err
			}
		}
		desc, text, err := richtext.Clean(req.Description)
		if err != nil {
			return err
		}
		if err := applyValues(ctx, q, project.ID, id, req.TypeID, req.CustomValues); err != nil {
			return err
		}
		updated, err := q.UpdateWorkItem(ctx, sqlc.UpdateWorkItemParams{
			ID: id, Version: req.Version, TypeID: req.TypeID, StatusID: req.StatusID, PriorityID: req.PriorityID,
			ParentID: req.ParentID, Title: req.Title, Description: desc, DescriptionText: text, AssigneeID: req.AssigneeID,
			StartDate: workcore.ToTime(req.StartDate), DueDate: workcore.ToTime(req.DueDate),
			EstimatedHours: req.EstimatedHours, RemainingHours: remaining, DoneRatio: ratio, Position: req.Position,
			Depth: row.Depth, ClosedAt: closedAt,
		})
		if err != nil {
			return fmt.Errorf("actualizar work item: %w", err)
		}
		if depth != row.Depth {
			if err := q.ShiftWorkItemDepths(ctx, sqlc.ShiftWorkItemDepthsParams{ID: id, Delta: int32(depth - row.Depth)}); err != nil {
				return fmt.Errorf("actualizar profundidad: %w", err)
			}
		}
		// Derived values of the work item, its ancestors and its old ancestors (RF-83).
		if err := q.RefreshWorkItemUp(ctx, id); err != nil {
			return fmt.Errorf("recalcular derivados: %w", err)
		}
		if parentChanged && row.ParentID != nil {
			if err := q.RefreshWorkItemUp(ctx, *row.ParentID); err != nil {
				return fmt.Errorf("recalcular derivados: %w", err)
			}
		}
		if change != unchanged && !project.IsTemplate {
			if err := emitStatusEvent(ctx, q, p, updated, key, change); err != nil {
				return err
			}
		}
		after := auditView(updated, key)
		after["description_changed"] = richtext.Changed(row.Description, desc)
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "work_item.updated", TargetType: "work_item",
			TargetID: id.String(), Before: auditView(row, key), After: after}); err != nil {
			return err
		}
		if updated, err = q.GetWorkItem(ctx, id); err != nil {
			return fmt.Errorf("releer work item: %w", err)
		}
		out, err = view(ctx, q, p, updated, project)
		return err
	})
	return out, err
}

func sameID(a, b *uuid.UUID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

type closeChange int

const (
	unchanged closeChange = iota
	closing
	reopening
	betweenClosed // closed to another closed status: no event (RF-118)
	betweenOpen
)

// statusChange checks a status change against the workflow (RF-86 to RF-88)
// and the blocking dependencies (RF-95), and says whether it closes or
// reopens the work item.
func statusChange(ctx context.Context, q *sqlc.Queries, p *authz.Principal, row sqlc.WorkItem, to uuid.UUID, key string) (closeChange, error) {
	if to == row.StatusID {
		return unchanged, nil
	}
	target, err := q.GetStatus(ctx, to)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && target.RetiredAt != nil) {
		return unchanged, apperr.ErrValidation.WithDetail("field", "status_id")
	}
	if err != nil {
		return unchanged, fmt.Errorf("leer estado: %w", err)
	}
	allowed, err := catalog.AllowedTargets(ctx, q, p, row.TypeID, row.StatusID)
	if err != nil {
		return unchanged, err
	}
	if !slices.Contains(allowed, to) {
		return unchanged, errTransition
	}
	wasClosed, err := q.StatusIsClosed(ctx, row.StatusID)
	if err != nil {
		return unchanged, fmt.Errorf("leer estado: %w", err)
	}
	switch {
	case target.IsClosed && !wasClosed:
		blockers, err := q.OpenBlockers(ctx, row.ID)
		if err != nil {
			return unchanged, fmt.Errorf("leer bloqueos: %w", err)
		}
		if len(blockers) > 0 {
			refs, err := refs(ctx, q, blockers)
			if err != nil {
				return unchanged, err
			}
			return unchanged, errBlocked.WithDetail("blocked_by", refs[blockers[0]].Key)
		}
		return closing, nil
	case !target.IsClosed && wasClosed:
		return reopening, nil
	case target.IsClosed:
		return betweenClosed, nil
	}
	return betweenOpen, nil
}

// emitStatusEvent records work_item.closed or work_item.reopened (RF-116,
// RF-117, RF-122) with the assignee at that moment.
func emitStatusEvent(ctx context.Context, q *sqlc.Queries, p *authz.Principal, row sqlc.WorkItem, key string, change closeChange) error {
	var typ string
	switch change {
	case closing:
		typ = events.WorkItemClosed
	case reopening:
		typ = events.WorkItemReopened
	default:
		return nil
	}
	actor := p.UserID
	return events.Emit(ctx, q, events.New{
		Type: typ, ActorID: &actor, ProjectID: row.ProjectID, SubjectType: "work_item", SubjectID: row.ID,
		Payload: map[string]any{"key": key, "status_id": row.StatusID, "assignee_id": row.AssigneeID, "type_id": row.TypeID},
	})
}

// Delete removes a work item without children (RF-85).
func (s *Service) Delete(ctx context.Context, p *authz.Principal, id uuid.UUID) error {
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, project, err := Load(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if !p.CanDeleteWorkItem(projects.Ref(project), row.AuthorID) {
			return apperr.ErrForbidden
		}
		if err := projects.Writable(project); err != nil {
			return err
		}
		// The tree lock keeps a concurrent child from appearing meanwhile.
		if err := q.LockWorkItemTree(ctx, project.ID); err != nil {
			return fmt.Errorf("bloquear árbol: %w", err)
		}
		children, err := q.CountWorkItemChildren(ctx, &id)
		if err != nil {
			return fmt.Errorf("contar hijos: %w", err)
		}
		if children > 0 {
			return errHasChildren
		}
		if err := q.DeleteWorkItem(ctx, id); err != nil {
			return fmt.Errorf("borrar work item: %w", err)
		}
		if row.ParentID != nil {
			if err := q.RefreshWorkItemUp(ctx, *row.ParentID); err != nil {
				return fmt.Errorf("recalcular derivados: %w", err)
			}
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "work_item.deleted", TargetType: "work_item",
			TargetID: id.String(), Before: auditView(row, Key(project.Identifier, row.SequenceNumber))})
	})
}

// Ancestors returns the ancestors of a work item, nearest first (RF-79).
func (s *Service) Ancestors(ctx context.Context, p *authz.Principal, id uuid.UUID) ([]Ref, error) {
	if _, _, err := Load(ctx, s.store.Queries, p, id, false); err != nil {
		return nil, err
	}
	ids, err := s.store.WorkItemAncestorIDs(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("leer ancestros: %w", err)
	}
	byID, err := refs(ctx, s.store.Queries, ids)
	if err != nil {
		return nil, err
	}
	out := make([]Ref, 0, len(ids))
	for _, a := range ids {
		out = append(out, byID[a])
	}
	return out, nil
}

// refs returns the key and title of some work items.
func refs(ctx context.Context, q *sqlc.Queries, ids []uuid.UUID) (map[uuid.UUID]Ref, error) {
	out := make(map[uuid.UUID]Ref, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := q.WorkItemRefs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("leer referencias: %w", err)
	}
	for _, r := range rows {
		out[r.ID] = Ref{ID: r.ID, Key: Key(r.Identifier, r.SequenceNumber), Title: r.Title}
	}
	return out, nil
}

// view builds the full work item for the caller.
func view(ctx context.Context, q *sqlc.Queries, p *authz.Principal, row sqlc.WorkItem, project sqlc.Project) (WorkItem, error) {
	ref := projects.Ref(project)
	out := WorkItem{
		ID: row.ID, Key: Key(project.Identifier, row.SequenceNumber), SequenceNumber: row.SequenceNumber,
		Project: ProjectRef{ID: project.ID, Identifier: project.Identifier, Name: project.Name},
		TypeID:  row.TypeID, StatusID: row.StatusID, PriorityID: row.PriorityID, Title: row.Title,
		Description: row.Description, Author: workcore.Ref{ID: row.AuthorID},
		StartDate: workcore.FromTime(row.StartDate), DueDate: workcore.FromTime(row.DueDate),
		EstimatedHours: row.EstimatedHours, RemainingHours: row.RemainingHours, DoneRatio: row.DoneRatio,
		Derived:  Derived{EstimatedHours: row.AggEstimated, RemainingHours: row.AggRemaining},
		Position: row.Position, Depth: row.Depth, Version: row.Version, ClosedAt: row.ClosedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, AllowedStatusIDs: []uuid.UUID{},
		Can: Can{Write: p.CanWriteWorkItems(ref), Delete: p.CanDeleteWorkItem(ref, row.AuthorID)},
	}
	if row.DerivedDoneRatio != nil {
		out.Derived.DoneRatio = *row.DerivedDoneRatio
	}
	people := []uuid.UUID{row.AuthorID}
	if row.AssigneeID != nil {
		people = append(people, *row.AssigneeID)
	}
	names, err := q.WorkItemPeople(ctx, people)
	if err != nil {
		return out, fmt.Errorf("leer personas: %w", err)
	}
	for _, n := range names {
		if n.ID == row.AuthorID {
			out.Author.Name = n.Name
		}
		if row.AssigneeID != nil && n.ID == *row.AssigneeID {
			out.Assignee = &workcore.Ref{ID: n.ID, Name: n.Name}
		}
	}
	if row.ParentID != nil {
		parents, err := refs(ctx, q, []uuid.UUID{*row.ParentID})
		if err != nil {
			return out, err
		}
		parent := parents[*row.ParentID]
		out.Parent = &parent
	}
	if out.ChildCount, err = q.CountWorkItemChildren(ctx, &row.ID); err != nil {
		return out, fmt.Errorf("contar hijos: %w", err)
	}
	values, err := q.ListWorkItemCustomValues(ctx, row.ID)
	if err != nil {
		return out, fmt.Errorf("leer valores: %w", err)
	}
	out.CustomValues = make(map[uuid.UUID]json.RawMessage, len(values))
	for _, v := range values {
		out.CustomValues[v.FieldID] = v.Value
	}
	if out.Can.Write && projects.Writable(project) == nil {
		if out.AllowedStatusIDs, err = catalog.AllowedTargets(ctx, q, p, row.TypeID, row.StatusID); err != nil {
			return out, err
		}
	}
	out.Dependencies, err = dependencies(ctx, q, row)
	return out, err
}

// auditView is what the audit keeps of a work item: the description only as
// a flag (RF-135).
func auditView(r sqlc.WorkItem, key string) map[string]any {
	return map[string]any{
		"key": key, "title": r.Title, "type_id": r.TypeID, "status_id": r.StatusID, "priority_id": r.PriorityID,
		"parent_id": r.ParentID, "assignee_id": r.AssigneeID, "start_date": workcore.FromTime(r.StartDate),
		"due_date": workcore.FromTime(r.DueDate), "estimated_hours": r.EstimatedHours,
		"remaining_hours": r.RemainingHours, "done_ratio": r.DoneRatio,
	}
}
