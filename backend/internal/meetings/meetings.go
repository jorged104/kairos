// Package meetings keeps the meetings of projects and programs (spec 018):
// a title, a date and time, a place or link and a page of notes, created
// blank or from a template. A meeting belongs to an owner; today only a
// project, and its permissions are the project's.
package meetings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/events"
	"openscum/internal/files"
	"openscum/internal/mentions"
	"openscum/internal/projects"
)

// OwnerProject is the only owner of a meeting for now (RF-2).
const OwnerProject = "project"

// Kind names a meeting as an owner of a page and of files; CommentKind owns
// the files of its comments.
const (
	Kind        = "meeting"
	CommentKind = "meeting_comment"
)

// PageSize bounds the lists of meetings (RF-40).
const PageSize = 50

var errVersion = func(current int32) *apperr.Error { return apperr.VersionConflict(current) }

type ProjectRef struct {
	ID         uuid.UUID `json:"id"`
	Identifier string    `json:"identifier"`
	Name       string    `json:"name"`
}

type Owner struct {
	Type string    `json:"type"`
	ID   uuid.UUID `json:"id"`
}

// Task is a work item that came from the meeting (RF-27).
type Task struct {
	ID       uuid.UUID `json:"id"`
	Key      string    `json:"key"`
	Title    string    `json:"title"`
	StatusID uuid.UUID `json:"status_id"`
}

type Can struct {
	Write   bool `json:"write"`
	Comment bool `json:"comment"`
}

// Meeting is a meeting with its notes and the work items that came from it.
type Meeting struct {
	ID          uuid.UUID               `json:"id"`
	Owner       Owner                   `json:"owner"`
	Project     ProjectRef              `json:"project"`
	Title       string                  `json:"title"`
	StartsAt    time.Time               `json:"starts_at"`
	Location    string                  `json:"location"`
	Description json.RawMessage         `json:"description"`
	PageVersion int32                   `json:"page_version"`
	Refs        map[string]mentions.Ref `json:"refs,omitempty"`
	Version     int32                   `json:"version"`
	TemplateID  *uuid.UUID              `json:"template_id"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	Tasks       []Task                  `json:"tasks"`
	Can         Can                     `json:"can"`
}

// Summary is a row of the list of a project's meetings (RF-39).
type Summary struct {
	ID       uuid.UUID `json:"id"`
	Title    string    `json:"title"`
	StartsAt time.Time `json:"starts_at"`
	Location string    `json:"location"`
	Tasks    int64     `json:"tasks"`
}

type Page struct {
	Items    []Summary `json:"items"`
	Total    int64     `json:"total"`
	Page     int       `json:"page"`
	PageSize int       `json:"page_size"`
}

type createRequest struct {
	Title      string     `json:"title" validate:"max=255"`
	StartsAt   *time.Time `json:"starts_at" validate:"required"`
	Location   *string    `json:"location" validate:"omitempty,max=500"`
	TemplateID *uuid.UUID `json:"template_id"`
	// Description goes after the template's page, if both come (RF-47).
	Description json.RawMessage `json:"description"`
}

func (r *createRequest) Normalize() { r.Title = strings.TrimSpace(r.Title) }

type updateRequest struct {
	Version  int32     `json:"version" validate:"required,min=1"`
	Title    string    `json:"title" validate:"required,max=255"`
	StartsAt time.Time `json:"starts_at" validate:"required"`
	Location string    `json:"location" validate:"max=500"`
}

func (r *updateRequest) Normalize() {
	r.Title = strings.TrimSpace(r.Title)
	r.Location = strings.TrimSpace(r.Location)
}

type Service struct {
	store *db.Store
	now   func() time.Time
}

func NewService(store *db.Store) *Service { return &Service{store: store, now: time.Now} }

// Load reads a meeting p can see, with the project it is checked against.
func Load(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID, lock bool) (sqlc.Meeting, sqlc.Project, error) {
	get := q.GetMeeting
	if lock {
		get = q.GetMeetingForUpdate
	}
	m, err := get(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, sqlc.Project{}, apperr.ErrNotFound
	}
	if err != nil {
		return m, sqlc.Project{}, fmt.Errorf("leer reunión: %w", err)
	}
	project, err := projects.Get(ctx, q, p, m.ProjectID, false)
	return m, project, err
}

// CanWrite: whoever writes the project's work items, while it is active (RF-8, RF-10).
func CanWrite(p *authz.Principal, project sqlc.Project) bool {
	return p.CanWriteWorkItems(projects.Ref(project)) && projects.Writable(project) == nil
}

func mayWrite(p *authz.Principal, project sqlc.Project) error {
	if !p.CanWriteWorkItems(projects.Ref(project)) {
		return apperr.ErrForbidden
	}
	return projects.Writable(project)
}

// Owner of the meeting's page, for its files.
func PageOwner(m sqlc.Meeting) files.Owner {
	return files.Owner{Kind: Kind, ID: m.ID, ProjectID: m.ProjectID}
}

func view(ctx context.Context, q *sqlc.Queries, p *authz.Principal, m sqlc.Meeting, project sqlc.Project) (Meeting, error) {
	out := Meeting{
		ID: m.ID, Owner: Owner{Type: m.OwnerType, ID: m.OwnerID},
		Project: ProjectRef{ID: project.ID, Identifier: project.Identifier, Name: project.Name},
		Title:   m.Title, StartsAt: m.StartsAt, Location: m.Location, Description: m.Description,
		PageVersion: m.PageVersion, Version: m.Version, TemplateID: m.TemplateID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		Tasks: []Task{},
		Can:   Can{Write: CanWrite(p, project), Comment: p.CanComment() && projects.Writable(project) == nil},
	}
	rows, err := q.MeetingTasks(ctx, &m.ID)
	if err != nil {
		return out, fmt.Errorf("leer tareas de la reunión: %w", err)
	}
	for _, r := range rows {
		out.Tasks = append(out.Tasks, Task{ID: r.ID, Key: fmt.Sprintf("%s-%d", r.Identifier, r.SequenceNumber), Title: r.Title, StatusID: r.StatusID})
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, p *authz.Principal, id uuid.UUID) (Meeting, error) {
	q := s.store.Queries
	m, project, err := Load(ctx, q, p, id, false)
	if err != nil {
		return Meeting{}, err
	}
	out, err := view(ctx, q, p, m, project)
	if err != nil {
		return out, err
	}
	out.Refs, err = mentions.Refs(ctx, q, p, out.Description)
	return out, err
}

// List is a page of a project's upcoming or past meetings (RF-38 to RF-40).
func (s *Service) List(ctx context.Context, p *authz.Principal, projectID uuid.UUID, upcoming bool, page, size int) (Page, error) {
	q := s.store.Queries
	if _, err := projects.Get(ctx, q, p, projectID, false); err != nil {
		return Page{}, err
	}
	now := s.now()
	rows, err := q.ListMeetings(ctx, sqlc.ListMeetingsParams{OwnerType: OwnerProject, OwnerID: projectID, Now: now, Upcoming: upcoming,
		RowLimit: int32(size), RowOffset: int32((page - 1) * size)})
	if err != nil {
		return Page{}, fmt.Errorf("listar reuniones: %w", err)
	}
	total, err := q.CountMeetings(ctx, sqlc.CountMeetingsParams{OwnerType: OwnerProject, OwnerID: projectID, Now: now, Upcoming: upcoming})
	if err != nil {
		return Page{}, fmt.Errorf("contar reuniones: %w", err)
	}
	out := Page{Items: make([]Summary, 0, len(rows)), Total: total, Page: page, PageSize: size}
	for _, r := range rows {
		out.Items = append(out.Items, Summary{ID: r.ID, Title: r.Title, StartsAt: r.StartsAt, Location: r.Location, Tasks: r.Tasks})
	}
	return out, nil
}

// Create adds a meeting to a project, blank or from a template (RF-1 to RF-8, RF-34, RF-35).
func (s *Service) Create(ctx context.Context, p *authz.Principal, projectID uuid.UUID, req createRequest) (Meeting, error) {
	var out Meeting
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		project, err := projects.Get(ctx, q, p, projectID, false)
		if err != nil {
			return err
		}
		if err := mayWrite(p, project); err != nil {
			return err
		}
		title, location := req.Title, ""
		if req.Location != nil {
			location = strings.TrimSpace(*req.Location)
		}
		var page json.RawMessage
		if req.TemplateID != nil {
			t, err := q.GetMeetingTemplate(ctx, *req.TemplateID)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && !usable(t, projectID)) {
				return apperr.ErrValidation.WithDetail("field", "template_id")
			}
			if err != nil {
				return fmt.Errorf("leer plantilla: %w", err)
			}
			if title == "" {
				title = t.Title
			}
			if req.Location == nil {
				location = t.Location
			}
			page = t.Page
		}
		if title == "" {
			return apperr.ErrValidation.WithDetail("field", "title") // RF-3
		}
		if len(req.Description) > 0 && string(req.Description) != "null" {
			page = joinDocs(page, req.Description)
		}
		desc, text, err := files.NewPage(page)
		if err != nil {
			return err
		}
		m, err := q.CreateMeeting(ctx, sqlc.CreateMeetingParams{
			OwnerType: OwnerProject, OwnerID: projectID, ProjectID: projectID, Title: title, StartsAt: *req.StartsAt,
			Location: location, Description: desc, DescriptionText: text, TemplateID: req.TemplateID, CreatedBy: p.UserID,
		})
		if err != nil {
			return fmt.Errorf("crear reunión: %w", err)
		}
		if !project.IsTemplate {
			if err := events.EmitMentions(ctx, q, p.UserID, projectID, Kind, m.ID, nil, desc); err != nil {
				return err
			}
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "meeting.created", TargetType: Kind,
			TargetID: m.ID.String(), After: auditView(m)}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, m, project)
		return err
	})
	return out, err
}

// usable: an active global template or one of the project (RF-34).
func usable(t sqlc.MeetingTemplate, projectID uuid.UUID) bool {
	if t.ProjectID == nil {
		return t.RetiredAt == nil
	}
	return *t.ProjectID == projectID
}

// joinDocs puts the blocks of b after those of a.
func joinDocs(a, b json.RawMessage) json.RawMessage {
	var x, y struct {
		Type    string            `json:"type"`
		Content []json.RawMessage `json:"content"`
	}
	if len(a) == 0 || json.Unmarshal(a, &x) != nil || x.Type != "doc" {
		return b
	}
	if json.Unmarshal(b, &y) != nil || y.Type != "doc" {
		return b
	}
	x.Content = append(x.Content, y.Content...)
	out, _ := json.Marshal(x)
	return out
}

func auditView(m sqlc.Meeting) map[string]any {
	return map[string]any{"title": m.Title, "starts_at": m.StartsAt, "location": m.Location, "project_id": m.ProjectID, "template_id": m.TemplateID}
}

// Update changes the title, the date and time or the place (RF-1, RF-15).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req updateRequest) (Meeting, error) {
	var out Meeting
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		m, project, err := Load(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if err := mayWrite(p, project); err != nil {
			return err
		}
		if req.Version != m.Version {
			return errVersion(m.Version)
		}
		updated, err := q.UpdateMeeting(ctx, sqlc.UpdateMeetingParams{ID: id, Version: req.Version, Title: req.Title, StartsAt: req.StartsAt, Location: req.Location})
		if err != nil {
			return fmt.Errorf("actualizar reunión: %w", err)
		}
		if err := audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "meeting.updated", TargetType: Kind,
			TargetID: id.String(), Before: auditView(m), After: auditView(updated)}); err != nil {
			return err
		}
		out, err = view(ctx, q, p, updated, project)
		return err
	})
	return out, err
}

// Delete removes a meeting with its comments; its files stop being served and
// its work items stay, without the link (RF-11 to RF-13).
func (s *Service) Delete(ctx context.Context, p *authz.Principal, id uuid.UUID) error {
	return s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		m, project, err := Load(ctx, q, p, id, true)
		if err != nil {
			return err
		}
		if err := mayWrite(p, project); err != nil {
			return err
		}
		for _, kind := range []string{Kind, CommentKind} {
			if err := q.MarkOwnerPageFilesDeleted(ctx, sqlc.MarkOwnerPageFilesDeletedParams{OwnerKind: kind, OwnerID: id}); err != nil {
				return fmt.Errorf("marcar archivos: %w", err)
			}
		}
		if err := q.DeleteMeeting(ctx, id); err != nil {
			return fmt.Errorf("borrar reunión: %w", err)
		}
		return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "meeting.deleted", TargetType: Kind,
			TargetID: id.String(), Before: auditView(m)})
	})
}

// errTimeZone is for an AI client that sends a date and time without its zone (RF-52).
var ErrTimeZone = apperr.New(http.StatusBadRequest, "meeting_time_zone", "La fecha y hora deben indicar su zona horaria")
