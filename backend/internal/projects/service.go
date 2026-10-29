package projects

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/customfields"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/files"
	"openscum/internal/richtext"
	"openscum/internal/workcore"
)

type Service struct {
	store *db.Store
	files *files.Service // copies the files of a template's pages (spec 005, RF-53)
}

func NewService(store *db.Store, filesSvc *files.Service) *Service {
	return &Service{store: store, files: filesSvc}
}

// Ref is what the permission checks need from a project.
func Ref(p sqlc.Project) authz.ProjectRef {
	return authz.ProjectRef{TeamID: p.TeamID, OwnerID: p.OwnerID}
}

// Writable rejects changes inside a closed or archived project (RF-29, RF-30).
func Writable(p sqlc.Project) error {
	switch p.Lifecycle {
	case LifecycleClosed:
		return ErrClosed
	case LifecycleArchived:
		return ErrArchived
	}
	return nil
}

// Visible hides templates from users that cannot use them (RF-35).
func Visible(p *authz.Principal, row sqlc.Project) bool {
	return !row.IsTemplate || p.Has(authz.TemplatesUse)
}

// Get loads a project the caller can see, optionally locking it.
func Get(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID, forUpdate bool) (sqlc.Project, error) {
	get := q.GetProject
	if forUpdate {
		get = q.GetProjectForUpdate
	}
	row, err := get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !Visible(p, row)) {
		return sqlc.Project{}, apperr.ErrNotFound
	}
	if err != nil {
		return sqlc.Project{}, fmt.Errorf("leer proyecto: %w", err)
	}
	return row, nil
}

func (s *Service) Get(ctx context.Context, p *authz.Principal, id uuid.UUID) (Project, error) {
	row, err := Get(ctx, s.store.Queries, p, id, false)
	if err != nil {
		return Project{}, err
	}
	return view(ctx, s.store.Queries, p, row)
}

// Create saves a new project with its types, fields and the phases of its
// methodology (RF-18 to RF-27, RF-32, RF-40).
func (s *Service) Create(ctx context.Context, p *authz.Principal, req createRequest) (Project, error) {
	var out Project
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := create(ctx, q, p, req, nil)
		if err != nil {
			return err
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project.created", TargetType: "project",
			TargetID: row.ID.String(), After: auditView(row)}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, row)
		return err
	})
	return out, err
}

// create inserts the project; templateID is set when it comes from a template.
func create(ctx context.Context, q *sqlc.Queries, p *authz.Principal, req createRequest, templateID *uuid.UUID) (sqlc.Project, error) {
	if !identifierRe.MatchString(req.Identifier) {
		return sqlc.Project{}, errIdentifierInvalid
	}
	if err := workcore.CheckRange(req.StartDate, req.EndDate, errDateRange); err != nil {
		return sqlc.Project{}, err
	}
	if !p.CanCreateProject(req.TeamID) {
		return sqlc.Project{}, apperr.ErrForbidden
	}
	if err := checkTeam(ctx, q, req.TeamID); err != nil {
		return sqlc.Project{}, err
	}
	owner := p.UserID
	if req.OwnerID != nil {
		owner = *req.OwnerID
	}
	if err := workcore.CheckWorker(ctx, q, owner, errInvalidOwner); err != nil {
		return sqlc.Project{}, err
	}
	if req.MethodologyID != nil {
		m, err := q.GetMethodology(ctx, *req.MethodologyID)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && m.RetiredAt != nil) {
			return sqlc.Project{}, apperr.ErrValidation.WithDetail("field", "methodology_id")
		}
		if err != nil {
			return sqlc.Project{}, fmt.Errorf("leer metodología: %w", err)
		}
	}
	depth, err := checkParent(ctx, q, p, uuid.Nil, req.ParentID, 0)
	if err != nil {
		return sqlc.Project{}, err
	}
	desc, text, err := files.NewPage(req.Description)
	if err != nil {
		return sqlc.Project{}, err
	}
	health := req.Health
	if health == "" {
		health = "en_curso"
	}
	row, err := q.CreateProject(ctx, sqlc.CreateProjectParams{
		ParentID: req.ParentID, TemplateID: templateID, TeamID: req.TeamID, OwnerID: owner,
		MethodologyID: req.MethodologyID, Identifier: req.Identifier, Name: req.Name,
		Description: desc, DescriptionText: text, Health: health, HealthExplanation: req.HealthExplanation,
		StartDate: workcore.ToTime(req.StartDate), EndDate: workcore.ToTime(req.EndDate), Depth: depth,
	})
	if auth.IsUniqueViolation(err) {
		return sqlc.Project{}, errIdentifierConflict
	}
	if err != nil {
		return sqlc.Project{}, fmt.Errorf("crear proyecto: %w", err)
	}

	types := req.TypeIDs
	if types == nil {
		all, err := q.ListWorkItemTypes(ctx)
		if err != nil {
			return sqlc.Project{}, fmt.Errorf("listar tipos: %w", err)
		}
		for _, t := range all {
			if t.RetiredAt == nil {
				types = append(types, t.ID)
			}
		}
	}
	if err := setTypes(ctx, q, row.ID, types); err != nil {
		return sqlc.Project{}, err
	}
	if err := setFields(ctx, q, row.ID, req.CustomFieldIDs); err != nil {
		return sqlc.Project{}, err
	}
	if err := applyValues(ctx, q, row.ID, req.CustomValues); err != nil {
		return sqlc.Project{}, err
	}
	if req.MethodologyID != nil {
		defs, err := q.ListActivePhaseDefinitions(ctx, *req.MethodologyID)
		if err != nil {
			return sqlc.Project{}, fmt.Errorf("leer fases: %w", err)
		}
		for i, d := range defs {
			if _, err := q.CreateProjectPhase(ctx, sqlc.CreateProjectPhaseParams{ProjectID: row.ID, DefinitionID: d.ID, Position: int32(i + 1)}); err != nil {
				return sqlc.Project{}, fmt.Errorf("crear fase: %w", err)
			}
		}
	}
	return row, nil
}

func checkTeam(ctx context.Context, q *sqlc.Queries, id uuid.UUID) error {
	t, err := q.GetTeam(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && t.ArchivedAt != nil) {
		return apperr.ErrValidation.WithDetail("field", "team_id")
	}
	if err != nil {
		return fmt.Errorf("leer equipo: %w", err)
	}
	return nil
}

// checkParent validates a (new) parent for project id (uuid.Nil when
// creating) whose subtree is height levels deep, and returns the new depth.
// It takes the project tree lock, so concurrent moves cannot build a cycle.
func checkParent(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID, parentID *uuid.UUID, height int32) (int16, error) {
	if parentID == nil {
		return 0, nil
	}
	if err := q.LockProjectTree(ctx); err != nil {
		return 0, fmt.Errorf("bloquear jerarquía: %w", err)
	}
	parent, err := q.GetProject(ctx, *parentID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && parent.IsTemplate) {
		return 0, apperr.ErrValidation.WithDetail("field", "parent_id")
	}
	if err != nil {
		return 0, fmt.Errorf("leer proyecto padre: %w", err)
	}
	if !p.CanManageProject(Ref(parent)) {
		return 0, apperr.ErrForbidden
	}
	if id != uuid.Nil {
		inside, err := q.ProjectIsDescendant(ctx, sqlc.ProjectIsDescendantParams{ProjectID: id, Candidate: *parentID})
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

// Update replaces the editable fields of a project (RF-18 to RF-30).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req updateRequest) (Project, error) {
	var out Project
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := Get(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if !p.CanManageProject(Ref(row)) {
			return apperr.ErrForbidden
		}
		if err := Writable(row); err != nil {
			return err
		}
		if req.Version != row.Version {
			return apperr.VersionConflict(row.Version)
		}
		if req.Identifier != nil && *req.Identifier != row.Identifier {
			return errIdentifierImmutable
		}
		if err := workcore.CheckRange(req.StartDate, req.EndDate, errDateRange); err != nil {
			return err
		}
		if req.TeamID != row.TeamID {
			if !p.CanCreateProject(req.TeamID) {
				return apperr.ErrForbidden
			}
			if err := checkTeam(ctx, q, req.TeamID); err != nil {
				return err
			}
		}
		if req.OwnerID != row.OwnerID {
			if err := workcore.CheckWorker(ctx, q, req.OwnerID, errInvalidOwner); err != nil {
				return err
			}
		}
		depth := row.Depth
		if !sameID(req.ParentID, row.ParentID) {
			height, err := q.ProjectSubtreeHeight(ctx, id)
			if err != nil {
				return fmt.Errorf("altura del subárbol: %w", err)
			}
			if depth, err = checkParent(ctx, q, p, id, req.ParentID, height); err != nil {
				return err
			}
		}
		desc, text, err := files.Description(ctx, q, files.Owner{Kind: "project", ID: id, ProjectID: id},
			req.Description, row.Description, row.DescriptionText)
		if err != nil {
			return err
		}
		if err := applyValues(ctx, q, id, req.CustomValues); err != nil {
			return err
		}
		updated, err := q.UpdateProject(ctx, sqlc.UpdateProjectParams{
			ID: id, Version: req.Version, ParentID: req.ParentID, TeamID: req.TeamID, OwnerID: req.OwnerID,
			Name: req.Name, Description: desc, DescriptionText: text, Health: req.Health,
			HealthExplanation: req.HealthExplanation, StartDate: workcore.ToTime(req.StartDate),
			EndDate: workcore.ToTime(req.EndDate), Depth: row.Depth,
		})
		if err != nil {
			return fmt.Errorf("actualizar proyecto: %w", err)
		}
		if depth != row.Depth {
			if err := q.ShiftProjectDepths(ctx, sqlc.ShiftProjectDepthsParams{ID: id, Delta: int32(depth - row.Depth)}); err != nil {
				return fmt.Errorf("actualizar profundidad: %w", err)
			}
			updated.Depth = depth
		}
		after := auditView(updated)
		after["description_changed"] = richtext.Changed(row.Description, desc)
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project.updated", TargetType: "project",
			TargetID: id.String(), Before: auditView(row), After: after}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, updated)
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

// lifecycleMoves lists the allowed lifecycle changes per action.
var lifecycleMoves = map[string]struct {
	from []string
	to   string
}{
	"close":     {[]string{LifecycleActive}, LifecycleClosed},
	"reopen":    {[]string{LifecycleClosed}, LifecycleActive},
	"archive":   {[]string{LifecycleActive, LifecycleClosed}, LifecycleArchived},
	"unarchive": {[]string{LifecycleArchived}, LifecycleActive},
}

func (s *Service) SetLifecycle(ctx context.Context, p *authz.Principal, id uuid.UUID, action string, version int32) (Project, error) {
	move, ok := lifecycleMoves[action]
	if !ok {
		return Project{}, apperr.ErrNotFound
	}
	var out Project
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := Get(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if !p.CanManageProject(Ref(row)) {
			return apperr.ErrForbidden
		}
		if version != row.Version {
			return apperr.VersionConflict(row.Version)
		}
		if !slices.Contains(move.from, row.Lifecycle) {
			return errLifecycle
		}
		updated, err := q.SetProjectLifecycle(ctx, sqlc.SetProjectLifecycleParams{ID: id, Version: version, Lifecycle: move.to})
		if err != nil {
			return fmt.Errorf("cambiar ciclo de vida: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "project." + action, TargetType: "project",
			TargetID: id.String(), Before: row.Lifecycle, After: move.to}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, updated)
		return err
	})
	return out, err
}

// SetTypes replaces the enabled work item types (RF-32, RF-33).
func (s *Service) SetTypes(ctx context.Context, p *authz.Principal, id uuid.UUID, ids []uuid.UUID) (Project, error) {
	return s.manage(ctx, p, id, "project.types_updated", func(q *sqlc.Queries, _ sqlc.Project) error {
		return setTypes(ctx, q, id, ids)
	})
}

// SetFields replaces the enabled custom fields (RF-15).
func (s *Service) SetFields(ctx context.Context, p *authz.Principal, id uuid.UUID, ids []uuid.UUID) (Project, error) {
	return s.manage(ctx, p, id, "project.fields_updated", func(q *sqlc.Queries, _ sqlc.Project) error {
		return setFields(ctx, q, id, ids)
	})
}

// manage runs a change of a writable project the caller manages, audits it
// and returns the new view.
func (s *Service) manage(ctx context.Context, p *authz.Principal, id uuid.UUID, action string, fn func(*sqlc.Queries, sqlc.Project) error) (Project, error) {
	var out Project
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		row, err := Get(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if !p.CanManageProject(Ref(row)) {
			return apperr.ErrForbidden
		}
		if err := Writable(row); err != nil {
			return err
		}
		if err := fn(q, row); err != nil {
			return err
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: action, TargetType: "project", TargetID: id.String()}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, row)
		return err
	})
	return out, err
}

func setTypes(ctx context.Context, q *sqlc.Queries, project uuid.UUID, ids []uuid.UUID) error {
	current, err := q.ListProjectTypeIDs(ctx, project)
	if err != nil {
		return fmt.Errorf("tipos del proyecto: %w", err)
	}
	for _, t := range current {
		if slices.Contains(ids, t) {
			continue
		}
		used, err := q.ProjectTypeInUse(ctx, sqlc.ProjectTypeInUseParams{ProjectID: project, TypeID: t})
		if err != nil {
			return fmt.Errorf("comprobar tipo: %w", err)
		}
		if used {
			return errTypeInUse.WithDetail("work_item_type_id", t.String())
		}
		if err := q.RemoveProjectType(ctx, sqlc.RemoveProjectTypeParams{ProjectID: project, TypeID: t}); err != nil {
			return fmt.Errorf("desactivar tipo: %w", err)
		}
	}
	for _, t := range ids {
		if slices.Contains(current, t) {
			continue
		}
		row, err := q.GetWorkItemType(ctx, t)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && row.RetiredAt != nil) {
			return apperr.ErrValidation.WithDetail("field", "work_item_type_ids")
		}
		if err != nil {
			return fmt.Errorf("leer tipo: %w", err)
		}
		if err := q.AddProjectType(ctx, sqlc.AddProjectTypeParams{ProjectID: project, TypeID: t}); err != nil {
			return fmt.Errorf("activar tipo: %w", err)
		}
	}
	return nil
}

func setFields(ctx context.Context, q *sqlc.Queries, project uuid.UUID, ids []uuid.UUID) error {
	current, err := customfields.EnabledFields(ctx, q, project)
	if err != nil {
		return err
	}
	if err := q.DeleteProjectCustomFields(ctx, project); err != nil {
		return fmt.Errorf("desactivar campos: %w", err)
	}
	for _, id := range ids {
		f, err := q.GetCustomField(ctx, id)
		// A retired field stays enabled if it already was, but cannot be added.
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && f.RetiredAt != nil && !current[id]) {
			return apperr.ErrValidation.WithDetail("field", "custom_field_ids")
		}
		if err != nil {
			return fmt.Errorf("leer campo: %w", err)
		}
		if err := q.AddProjectCustomField(ctx, sqlc.AddProjectCustomFieldParams{ProjectID: project, FieldID: id}); err != nil {
			return fmt.Errorf("activar campo: %w", err)
		}
	}
	return nil
}

func applyValues(ctx context.Context, q *sqlc.Queries, project uuid.UUID, values customfields.Values) error {
	defs, err := customfields.Load(ctx, q)
	if err != nil {
		return err
	}
	enabled, err := customfields.EnabledFields(ctx, q, project)
	if err != nil {
		return err
	}
	_, err = customfields.Apply(ctx, defs, customfields.ProjectValues(q, project),
		customfields.Scope{Entity: customfields.EntityProject, Enabled: enabled}, values, true)
	return err
}

// Subtree returns every descendant of a project (RF-28).
func (s *Service) Subtree(ctx context.Context, p *authz.Principal, id uuid.UUID) ([]SubtreeNode, error) {
	if _, err := Get(ctx, s.store.Queries, p, id, false); err != nil {
		return nil, err
	}
	rows, err := s.store.ProjectSubtree(ctx, &id)
	if err != nil {
		return nil, fmt.Errorf("subárbol: %w", err)
	}
	out := make([]SubtreeNode, 0, len(rows))
	for _, r := range rows {
		if r.IsTemplate && !p.Has(authz.TemplatesUse) {
			continue
		}
		out = append(out, SubtreeNode{ID: r.ID, ParentID: r.ParentID, Identifier: r.Identifier, Name: r.Name,
			Lifecycle: r.Lifecycle, Health: r.Health, Depth: r.Depth})
	}
	return out, nil
}

// view builds the full project for the caller.
func view(ctx context.Context, q *sqlc.Queries, p *authz.Principal, row sqlc.Project) (Project, error) {
	out := Project{
		ID: row.ID, Identifier: row.Identifier, Name: row.Name, Description: row.Description, PageVersion: row.PageVersion,
		TemplateID: row.TemplateID, IsTemplate: row.IsTemplate, Lifecycle: row.Lifecycle, Health: row.Health,
		HealthExplanation: row.HealthExplanation, StartDate: workcore.FromTime(row.StartDate), EndDate: workcore.FromTime(row.EndDate),
		Depth: row.Depth, Version: row.Version, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Team: workcore.Ref{ID: row.TeamID}, Owner: workcore.Ref{ID: row.OwnerID},
		Can: Can{
			Manage:         p.CanManageProject(Ref(row)),
			WriteWorkItems: p.CanWriteWorkItems(Ref(row)),
			ManageTemplate: p.Has(authz.TemplatesManage),
		},
	}
	names, err := q.ProjectNames(ctx, []uuid.UUID{row.ID})
	if err != nil {
		return out, fmt.Errorf("nombres del proyecto: %w", err)
	}
	if len(names) == 1 {
		n := names[0]
		out.Team.Name, out.Owner.Name = n.TeamName, n.OwnerName
		if row.MethodologyID != nil && n.MethodologyName != nil {
			out.Methodology = &workcore.Ref{ID: *row.MethodologyID, Name: *n.MethodologyName}
		}
		if row.ParentID != nil && n.ParentIdentifier != nil {
			out.Parent = &ParentRef{ID: *row.ParentID, Identifier: *n.ParentIdentifier, Name: *n.ParentName}
		}
	}
	if out.TypeIDs, err = q.ListProjectTypeIDs(ctx, row.ID); err != nil {
		return out, fmt.Errorf("tipos del proyecto: %w", err)
	}
	if out.CustomFieldIDs, err = q.ListProjectCustomFieldIDs(ctx, row.ID); err != nil {
		return out, fmt.Errorf("campos del proyecto: %w", err)
	}
	// Empty lists go out as [] rather than null.
	if out.TypeIDs == nil {
		out.TypeIDs = []uuid.UUID{}
	}
	if out.CustomFieldIDs == nil {
		out.CustomFieldIDs = []uuid.UUID{}
	}
	values, err := q.ListProjectCustomValues(ctx, row.ID)
	if err != nil {
		return out, fmt.Errorf("valores del proyecto: %w", err)
	}
	out.CustomValues = make(map[uuid.UUID]json.RawMessage, len(values))
	for _, v := range values {
		out.CustomValues[v.FieldID] = v.Value
	}
	return out, nil
}

// auditView is what the audit keeps of a project: the description only as a
// flag (RF-135).
func auditView(r sqlc.Project) map[string]any {
	return map[string]any{
		"identifier": r.Identifier, "name": r.Name, "team_id": r.TeamID, "owner_id": r.OwnerID,
		"parent_id": r.ParentID, "methodology_id": r.MethodologyID, "lifecycle": r.Lifecycle,
		"health": r.Health, "health_explanation": r.HealthExplanation, "start_date": workcore.FromTime(r.StartDate),
		"end_date": workcore.FromTime(r.EndDate), "is_template": r.IsTemplate,
	}
}
