package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/auth"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/workcore"
	"openscum/internal/workitems"
)

const (
	// DropdownSize is what the bell shows (RF-27); PageSize, the page (RF-29).
	DropdownSize = 20
	PageSize     = 50
	// shownActors are the names a notification shows; the rest are «N más» (RF-23).
	shownActors = 2
)

// Notification is what a person sees of one (RF-23, RF-24).
type Notification struct {
	ID          uuid.UUID       `json:"id"`
	Subject     Subject         `json:"subject"`
	Actors      []workcore.Ref  `json:"actors"`
	ActorsTotal int             `json:"actors_total"`
	Events      int32           `json:"events"`
	Mention     bool            `json:"mention"`
	Last        Last            `json:"last"`
	Via         json.RawMessage `json:"via"`
	ReadAt      *time.Time      `json:"read_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Subject is the element: a work item by key, a milestone or a project.
type Subject struct {
	Type        string    `json:"type"`
	ID          uuid.UUID `json:"id"`
	Key         string    `json:"key,omitempty"`
	Title       string    `json:"title"`
	ProjectID   uuid.UUID `json:"project_id"`
	ProjectName string    `json:"project_name"`
}

// Last is the last event: a comment with its excerpt, a status change with
// the names of both statuses, a mention or a thread resolved or reopened.
type Last struct {
	Kind      string     `json:"kind"`
	CommentID *uuid.UUID `json:"comment_id,omitempty"`
	Excerpt   string     `json:"excerpt,omitempty"`
	From      string     `json:"from,omitempty"`
	To        string     `json:"to,omitempty"`
}

type Page struct {
	Items    []Notification `json:"items"`
	Total    int64          `json:"total"`
	Unread   int64          `json:"unread"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

type Service struct {
	store  *db.Store
	hub    Sender
	logger *slog.Logger
}

func NewService(store *db.Store, hub Sender, logger *slog.Logger) *Service {
	return &Service{store: store, hub: hub, logger: logger}
}

func (s *Service) list(ctx context.Context, user uuid.UUID, unread bool, id *uuid.UUID, limit, offset int32) ([]Notification, error) {
	q := s.store.Queries
	rows, err := q.ListNotifications(ctx, sqlc.ListNotificationsParams{UserID: user, UnreadOnly: unread, ID: id, RowLimit: limit, RowOffset: offset})
	if err != nil {
		return nil, fmt.Errorf("listar notificaciones: %w", err)
	}
	var ids []uuid.UUID
	for _, r := range rows {
		ids = append(ids, r.Actors[:min(len(r.Actors), shownActors)]...)
	}
	names := map[uuid.UUID]string{}
	if len(ids) > 0 {
		users, err := q.UserNames(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("leer nombres: %w", err)
		}
		for _, u := range users {
			names[u.ID] = u.Name
		}
	}
	out := make([]Notification, 0, len(rows))
	for _, r := range rows {
		n := Notification{
			ID: r.ID, ActorsTotal: len(r.Actors), Events: r.Events, Mention: r.Mention, Via: r.Via, ReadAt: r.ReadAt, UpdatedAt: r.UpdatedAt,
			Subject: Subject{Type: r.SubjectType, ID: r.SubjectID, Title: r.Title, ProjectID: r.ProjectID, ProjectName: r.ProjectName},
			Actors:  []workcore.Ref{},
		}
		if r.SequenceNumber != nil {
			n.Subject.Key = r.ProjectIdentifier + "-" + strconv.Itoa(int(*r.SequenceNumber))
		}
		for _, a := range r.Actors[:min(len(r.Actors), shownActors)] {
			n.Actors = append(n.Actors, workcore.Ref{ID: a, Name: names[a]})
		}
		var last Last
		_ = json.Unmarshal(r.LastData, &last)
		last.Kind = r.LastKind
		if r.CommentGone {
			// A deleted comment: the notification leads to the work item, without its text.
			last.CommentID, last.Excerpt = nil, ""
		}
		n.Last = last
		out = append(out, n)
	}
	return out, nil
}

func (s *Service) unread(ctx context.Context, user uuid.UUID) (int64, error) {
	n, err := s.store.CountNotifications(ctx, sqlc.CountNotificationsParams{UserID: user, UnreadOnly: true})
	if err != nil {
		return 0, fmt.Errorf("contar notificaciones: %w", err)
	}
	return n, nil
}

// List is a page of p's notifications, unread first (RF-27 to RF-29).
func (s *Service) List(ctx context.Context, p *authz.Principal, unread bool, page, size int) (Page, error) {
	items, err := s.list(ctx, p.UserID, unread, nil, int32(size), int32((page-1)*size))
	if err != nil {
		return Page{}, err
	}
	total, err := s.store.CountNotifications(ctx, sqlc.CountNotificationsParams{UserID: p.UserID, UnreadOnly: unread})
	if err != nil {
		return Page{}, fmt.Errorf("contar notificaciones: %w", err)
	}
	n, err := s.unread(ctx, p.UserID)
	return Page{Items: items, Total: total, Unread: n, Page: page, PageSize: size}, err
}

// Unread is the number on the bell (RF-26).
func (s *Service) Unread(ctx context.Context, p *authz.Principal) (int64, error) {
	return s.unread(ctx, p.UserID)
}

// MarkRead marks one notification read or unread (RF-40, RF-42). If the
// element already has another unread one, it stays as it was: the element
// shows as unread anyway.
func (s *Service) MarkRead(ctx context.Context, p *authz.Principal, id uuid.UUID, read bool) error {
	exists, err := s.store.NotificationExists(ctx, sqlc.NotificationExistsParams{ID: id, UserID: p.UserID})
	if err != nil {
		return fmt.Errorf("leer notificación: %w", err)
	}
	if !exists {
		return apperr.ErrNotFound
	}
	if _, err := s.store.SetNotificationRead(ctx, sqlc.SetNotificationReadParams{ID: id, UserID: p.UserID, Read: read}); err != nil && !auth.IsUniqueViolation(err) {
		return fmt.Errorf("marcar notificación: %w", err)
	}
	s.push(ctx, p.UserID, nil)
	return nil
}

// ReadAll marks every notification of p read (RF-43).
func (s *Service) ReadAll(ctx context.Context, p *authz.Principal) error {
	if err := s.store.ReadAllNotifications(ctx, p.UserID); err != nil {
		return fmt.Errorf("marcar notificaciones: %w", err)
	}
	s.push(ctx, p.UserID, nil)
	return nil
}

// push tells the user's tabs the new count and, for a new or grown
// notification, the notification itself (RF-33 to RF-35).
func (s *Service) push(ctx context.Context, user uuid.UUID, id *uuid.UUID) {
	n, err := s.unread(ctx, user)
	if err != nil {
		s.logger.Error("contar notificaciones", slog.Any("error", err))
		return
	}
	msg := map[string]any{"type": "notifications", "unread": n}
	if id != nil {
		items, err := s.list(ctx, user, false, id, 1, 0)
		if err != nil {
			s.logger.Error("leer notificación", slog.Any("error", err))
			return
		}
		if len(items) == 0 {
			return
		}
		msg["notification"] = items[0]
	}
	s.hub.Send(user, msg)
}

// Following tells whether p follows a work item p can see (RF-2).
func (s *Service) Following(ctx context.Context, p *authz.Principal, workItem uuid.UUID) (bool, error) {
	if _, _, err := workitems.Load(ctx, s.store.Queries, p, workItem, false); err != nil {
		return false, err
	}
	following, err := s.store.GetFollow(ctx, sqlc.GetFollowParams{WorkItemID: workItem, UserID: p.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("leer seguimiento: %w", err)
	}
	return following, nil
}

// SetFollowing follows or stops following a work item p can see (RF-3, RF-4).
func (s *Service) SetFollowing(ctx context.Context, p *authz.Principal, workItem uuid.UUID, following bool) error {
	if _, _, err := workitems.Load(ctx, s.store.Queries, p, workItem, false); err != nil {
		return err
	}
	if err := s.store.SetFollow(ctx, sqlc.SetFollowParams{WorkItemID: workItem, UserID: p.UserID, Following: following}); err != nil {
		return fmt.Errorf("guardar seguimiento: %w", err)
	}
	return nil
}
