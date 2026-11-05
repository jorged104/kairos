// Package comments keeps the comments of work items (spec 006, RF-22 to
// RF-55) and of meetings (spec 018, RF-20): threads with one level of
// replies that can be resolved, and threads anchored to a piece of the page.
package comments

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/events"
	"openscum/internal/files"
	"openscum/internal/meetings"
	"openscum/internal/mentions"
	"openscum/internal/pages"
	"openscum/internal/projects"
	"openscum/internal/richtext"
	"openscum/internal/workcore"
	"openscum/internal/workitems"
)

// OwnerKind owns the files of the comments of a work item, by its id.
const OwnerKind = "work_item_comment"

// What comments are on.
const (
	KindWorkItem = "work_item"
	KindMeeting  = meetings.Kind
)

// MaxRunes bounds the text of a comment (RF-27).
const MaxRunes = 10_000

var (
	errEmpty       = apperr.New(http.StatusBadRequest, "comment_empty", "El comentario está vacío")
	errTooLong     = apperr.New(http.StatusBadRequest, "comment_too_long", "El comentario supera los 10 000 caracteres")
	errPageChanged = apperr.New(http.StatusConflict, "anchor_page_changed", "Al comentar solo se puede marcar el texto, no cambiar la página")
)

func pageConflict(current int32) *apperr.Error {
	return apperr.New(http.StatusConflict, "page_conflict", "La página cambió desde que la abriste").
		WithDetail("page_version", strconv.Itoa(int(current)))
}

type Service struct {
	store *db.Store
	files *files.Service
}

func NewService(store *db.Store, filesSvc *files.Service) *Service {
	return &Service{store: store, files: filesSvc}
}

type Can struct {
	Edit   bool `json:"edit"`
	Delete bool `json:"delete"`
}

type Comment struct {
	ID        uuid.UUID       `json:"id"`
	Author    workcore.Ref    `json:"author"`
	Body      json.RawMessage `json:"body"` // null once deleted (RF-33)
	CreatedAt time.Time       `json:"created_at"`
	EditedAt  *time.Time      `json:"edited_at"`
	Deleted   bool            `json:"deleted"`
	// Via is the AI connection that wrote it, null from the interface (spec 016, RF-44).
	Via json.RawMessage `json:"via"`
	Can Can             `json:"can"`
}

type Thread struct {
	Comment
	// Quote is the commented text of an anchored thread (RF-48).
	Quote    *string `json:"quote"`
	Anchored bool    `json:"anchored"`
	// AnchorDeleted: the commented text is no longer in the page (RF-51).
	AnchorDeleted bool          `json:"anchor_deleted"`
	ResolvedAt    *time.Time    `json:"resolved_at"`
	ResolvedBy    *workcore.Ref `json:"resolved_by"`
	Replies       []Comment     `json:"replies"`
}

// List is every thread of a work item or a meeting, oldest first (RF-29),
// with the mentions of all of them.
type List struct {
	Threads    []Thread                `json:"threads"`
	Refs       map[string]mentions.Ref `json:"refs"`
	CanComment bool                    `json:"can_comment"`
}

// PageSaved is the page after anchoring a thread in it.
type PageSaved struct {
	Page        json.RawMessage         `json:"page"`
	PageVersion int32                   `json:"page_version"`
	Version     int32                   `json:"version"`
	Refs        map[string]mentions.Ref `json:"refs"`
}

// Result answers every change with the comments as they are now (RF-43).
type Result struct {
	Comments List       `json:"comments"`
	Page     *PageSaved `json:"page,omitempty"`
}

type anchorRequest struct {
	ThreadID    uuid.UUID       `json:"thread_id" validate:"required"`
	Quote       string          `json:"quote" validate:"required,max=1000"`
	Page        json.RawMessage `json:"page"`
	PageVersion int32           `json:"page_version" validate:"required,min=1"`
}

type createRequest struct {
	Body json.RawMessage `json:"body"`
	// ThreadID answers a thread; a reply to a reply goes to the same thread (RF-28).
	ThreadID *uuid.UUID     `json:"thread_id"`
	Anchor   *anchorRequest `json:"anchor"`
}

type updateRequest struct {
	Body json.RawMessage `json:"body"`
}

// target is what the comments are on, with what they need of it: its
// project for the permissions, and its page for the anchored threads.
type target struct {
	Kind        string
	ID          uuid.UUID
	Project     sqlc.Project
	Page        json.RawMessage
	PageVersion int32
}

// load reads the target, which also checks the caller can read it.
func load(ctx context.Context, q *sqlc.Queries, p *authz.Principal, kind string, id uuid.UUID, lock bool) (target, error) {
	switch kind {
	case KindWorkItem:
		w, project, err := workitems.Load(ctx, q, p, id, lock)
		return target{Kind: kind, ID: id, Project: project, Page: w.Description, PageVersion: w.PageVersion}, err
	case KindMeeting:
		m, project, err := meetings.Load(ctx, q, p, id, lock)
		return target{Kind: kind, ID: id, Project: project, Page: m.Description, PageVersion: m.PageVersion}, err
	}
	return target{}, apperr.ErrNotFound
}

// of is the target of a stored comment.
func of(c sqlc.WorkItemComment) (string, uuid.UUID) {
	if c.MeetingID != nil {
		return KindMeeting, *c.MeetingID
	}
	return KindWorkItem, *c.WorkItemID
}

func (t target) is(c sqlc.WorkItemComment) bool {
	kind, id := of(c)
	return kind == t.Kind && id == t.ID
}

// filesOwner owns the files of the target's comments.
func (t target) filesOwner() files.Owner {
	kind := OwnerKind
	if t.Kind == KindMeeting {
		kind = meetings.CommentKind
	}
	return files.Owner{Kind: kind, ID: t.ID, ProjectID: t.Project.ID}
}

// canComment: every role but viewer, in an active project (RF-23, RF-39).
func canComment(p *authz.Principal, project sqlc.Project) bool {
	return p.CanComment() && projects.Writable(project) == nil
}

func mayComment(p *authz.Principal, project sqlc.Project) error {
	if !p.CanComment() {
		return apperr.ErrForbidden
	}
	return projects.Writable(project)
}

// body validates the body of a comment against the files of its target's comments.
func body(ctx context.Context, q *sqlc.Queries, t target, raw json.RawMessage) (json.RawMessage, string, error) {
	owned, err := files.Owned(ctx, q, t.filesOwner())
	if err != nil {
		return nil, "", err
	}
	doc, text, _, err := richtext.CleanComment(raw, owned)
	if err != nil {
		return nil, "", err
	}
	if doc == nil {
		return nil, "", errEmpty
	}
	if utf8.RuneCountInString(text) > MaxRunes {
		return nil, "", errTooLong
	}
	return doc, text, nil
}

func record(ctx context.Context, q *sqlc.Queries, p *authz.Principal, t target, action string, id uuid.UUID, after map[string]any) error {
	if after == nil {
		after = map[string]any{}
	}
	after[t.Kind+"_id"] = t.ID
	// Never the text (RF-42).
	return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: t.Kind + ".comment_" + action,
		TargetType: t.Kind + "_comment", TargetID: id.String(), After: after})
}

// List returns the threads of a work item or a meeting.
func (s *Service) List(ctx context.Context, p *authz.Principal, kind string, id uuid.UUID) (List, error) {
	t, err := load(ctx, s.store.Queries, p, kind, id, false)
	if err != nil {
		return List{}, err
	}
	return list(ctx, s.store.Queries, p, t)
}

func rowsOf(ctx context.Context, q *sqlc.Queries, t target) ([]sqlc.ListCommentsRow, error) {
	if t.Kind == KindMeeting {
		rows, err := q.ListMeetingComments(ctx, &t.ID)
		out := make([]sqlc.ListCommentsRow, len(rows))
		for i, r := range rows {
			out[i] = sqlc.ListCommentsRow(r)
		}
		return out, err
	}
	return q.ListComments(ctx, &t.ID)
}

func list(ctx context.Context, q *sqlc.Queries, p *authz.Principal, t target) (List, error) {
	rows, err := rowsOf(ctx, q, t)
	if err != nil {
		return List{}, fmt.Errorf("leer comentarios: %w", err)
	}
	can := canComment(p, t.Project)
	manager := p.CanManageProject(projects.Ref(t.Project))
	live := richtext.Threads(t.Page)
	out := List{Threads: []Thread{}, CanComment: can}
	at := map[uuid.UUID]int{}
	var bodies [][]byte
	for _, r := range rows {
		mine := r.AuthorID == p.UserID
		deleted := r.DeletedAt != nil
		c := Comment{
			ID: r.ID, Author: workcore.Ref{ID: r.AuthorID, Name: r.AuthorName}, Body: r.Body,
			CreatedAt: r.CreatedAt, EditedAt: r.EditedAt, Deleted: deleted, Via: r.Via,
			Can: Can{Edit: can && mine && !deleted, Delete: can && (mine || manager) && !deleted},
		}
		if len(r.Body) > 0 {
			bodies = append(bodies, r.Body)
		}
		if r.ThreadID == nil {
			th := Thread{Comment: c, Quote: r.Quote, Anchored: r.Anchored, AnchorDeleted: r.Anchored && !live[r.ID],
				ResolvedAt: r.ResolvedAt, Replies: []Comment{}}
			if r.ResolvedBy != nil && r.ResolvedByName != nil {
				th.ResolvedBy = &workcore.Ref{ID: *r.ResolvedBy, Name: *r.ResolvedByName}
			}
			at[r.ID] = len(out.Threads)
			out.Threads = append(out.Threads, th)
			continue
		}
		if i, ok := at[*r.ThreadID]; ok {
			out.Threads[i].Replies = append(out.Threads[i].Replies, c)
		}
	}
	// One lookup for the mentions of every comment.
	all := append([]byte(`{"type":"doc","content":[`), bytes.Join(bodies, []byte(","))...)
	all = append(all, "]}"...)
	if out.Refs, err = mentions.Refs(ctx, q, p, all); err != nil {
		return List{}, err
	}
	return out, nil
}

// Create adds a thread, a reply or a thread anchored in the page.
func (s *Service) Create(ctx context.Context, p *authz.Principal, kind string, id uuid.UUID, req createRequest) (Result, error) {
	var out Result
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := load(ctx, q, p, kind, id, true)
		if err != nil {
			return err
		}
		if err := mayComment(p, t.Project); err != nil {
			return err
		}
		doc, text, err := body(ctx, q, t, req.Body)
		if err != nil {
			return err
		}
		via, err := audit.ViaJSON(ctx)
		if err != nil {
			return err
		}
		params := sqlc.CreateCommentParams{AuthorID: p.UserID, Body: doc, BodyText: text, Via: via}
		if t.Kind == KindMeeting {
			params.MeetingID = &t.ID
		} else {
			params.WorkItemID = &t.ID
		}
		after := map[string]any{}
		switch {
		case req.ThreadID != nil && req.Anchor != nil:
			return apperr.ErrValidation.WithDetail("field", "anchor")
		case req.ThreadID != nil:
			root, err := thread(ctx, q, t, *req.ThreadID)
			if err != nil {
				return err
			}
			params.ThreadID = &root.ID
			after["thread_id"] = root.ID
			// Answering a resolved thread reopens it (RF-38).
			if root.ResolvedAt != nil {
				if _, err := q.SetThreadResolved(ctx, sqlc.SetThreadResolvedParams{ID: root.ID, Resolved: false}); err != nil {
					return fmt.Errorf("reabrir hilo: %w", err)
				}
				after["reopened"] = true
			}
		case req.Anchor != nil:
			saved, err := anchor(ctx, q, p, t, *req.Anchor)
			if err != nil {
				return err
			}
			out.Page = &saved
			params.ID, params.Quote, params.Anchored = &req.Anchor.ThreadID, &req.Anchor.Quote, true
			after["anchored"] = true
			if t, err = load(ctx, q, p, kind, id, false); err != nil {
				return err
			}
		}
		c, err := q.CreateComment(ctx, params)
		if err != nil {
			return fmt.Errorf("crear comentario: %w", err)
		}
		if err := record(ctx, q, p, t, "created", c.ID, after); err != nil {
			return err
		}
		if err := emitCreated(ctx, q, p, t, c, text); err != nil {
			return err
		}
		out.Comments, err = list(ctx, q, p, t)
		return err
	})
	return out, err
}

// thread returns the root of the thread of comment id, in target t.
func thread(ctx context.Context, q *sqlc.Queries, t target, id uuid.UUID) (sqlc.WorkItemComment, error) {
	c, err := q.GetCommentForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && !t.is(c)) {
		return c, apperr.ErrNotFound
	}
	if err != nil {
		return c, fmt.Errorf("leer comentario: %w", err)
	}
	if c.ThreadID != nil {
		return thread(ctx, q, t, *c.ThreadID)
	}
	return c, nil
}

// anchor saves the page with the new thread's mark. The page must be the
// stored one, with the same anchors plus the new one: whoever can't edit the
// page only marks it (RF-46).
func anchor(ctx context.Context, q *sqlc.Queries, p *authz.Principal, t target, a anchorRequest) (PageSaved, error) {
	if a.PageVersion != t.PageVersion {
		return PageSaved{}, pageConflict(t.PageVersion)
	}
	exists, err := q.CommentExists(ctx, a.ThreadID)
	if err != nil {
		return PageSaved{}, fmt.Errorf("comprobar hilo: %w", err)
	}
	if exists {
		return PageSaved{}, apperr.ErrValidation.WithDetail("field", "thread_id")
	}
	doc, text, page, err := files.Page(ctx, q, files.Owner{Kind: t.Kind, ID: t.ID, ProjectID: t.Project.ID}, a.Page)
	if err != nil {
		return PageSaved{}, err
	}
	before := richtext.Threads(t.Page)
	if !slices.Contains(page.Threads, a.ThreadID) || len(page.Threads) != len(before)+1 {
		return PageSaved{}, errPageChanged
	}
	for _, id := range page.Threads {
		if id != a.ThreadID && !before[id] {
			return PageSaved{}, errPageChanged
		}
	}
	if !richtext.SameButAnchors(doc, t.Page) {
		return PageSaved{}, errPageChanged
	}
	saved, err := pages.Update(ctx, q, t.Kind, t.ID, a.PageVersion, doc, text)
	if err != nil {
		return PageSaved{}, err
	}
	refs, err := mentions.Refs(ctx, q, p, saved.Page)
	if err != nil {
		return PageSaved{}, err
	}
	return PageSaved{Page: saved.Page, PageVersion: saved.PageVersion, Version: saved.Version, Refs: refs}, nil
}

// comment loads a comment with its target, for a change.
func comment(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.WorkItemComment, target, error) {
	c, err := q.GetCommentForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, target{}, apperr.ErrNotFound
	}
	if err != nil {
		return c, target{}, fmt.Errorf("leer comentario: %w", err)
	}
	kind, owner := of(c)
	t, err := load(ctx, q, p, kind, owner, false)
	if err != nil {
		return c, t, err
	}
	if err := mayComment(p, t.Project); err != nil {
		return c, t, err
	}
	return c, t, nil
}

// Update changes the text of one's own comment (RF-31, RF-35).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req updateRequest) (Result, error) {
	var out Result
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		c, t, err := comment(ctx, q, p, id)
		if err != nil {
			return err
		}
		if c.DeletedAt != nil {
			return apperr.ErrNotFound
		}
		if c.AuthorID != p.UserID {
			return apperr.ErrForbidden
		}
		doc, text, err := body(ctx, q, t, req.Body)
		if err != nil {
			return err
		}
		if _, err := q.UpdateCommentBody(ctx, sqlc.UpdateCommentBodyParams{ID: id, Body: doc, BodyText: text}); err != nil {
			return fmt.Errorf("editar comentario: %w", err)
		}
		if err := record(ctx, q, p, t, "updated", id, nil); err != nil {
			return err
		}
		out.Comments, err = list(ctx, q, p, t)
		return err
	})
	return out, err
}

// Delete removes a comment: its author, or whoever manages the project
// (RF-32 to RF-34). A root with replies stays as «Comentario eliminado».
func (s *Service) Delete(ctx context.Context, p *authz.Principal, id uuid.UUID) (Result, error) {
	var out Result
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		c, t, err := comment(ctx, q, p, id)
		if err != nil {
			return err
		}
		if c.DeletedAt != nil {
			return apperr.ErrNotFound
		}
		if c.AuthorID != p.UserID && !p.CanManageProject(projects.Ref(t.Project)) {
			return apperr.ErrForbidden
		}
		if err := remove(ctx, q, c); err != nil {
			return err
		}
		if err := record(ctx, q, p, t, "deleted", id, map[string]any{"author_id": c.AuthorID}); err != nil {
			return err
		}
		out.Comments, err = list(ctx, q, p, t)
		return err
	})
	return out, err
}

func remove(ctx context.Context, q *sqlc.Queries, c sqlc.WorkItemComment) error {
	if c.ThreadID == nil {
		replies, err := q.CountReplies(ctx, &c.ID)
		if err != nil {
			return fmt.Errorf("contar respuestas: %w", err)
		}
		if replies > 0 {
			if err := q.BlankComment(ctx, c.ID); err != nil {
				return fmt.Errorf("borrar comentario: %w", err)
			}
			return nil
		}
	}
	if err := q.DeleteComment(ctx, c.ID); err != nil {
		return fmt.Errorf("borrar comentario: %w", err)
	}
	if c.ThreadID == nil {
		return nil
	}
	// The last reply of a deleted root takes the thread with it.
	root, err := q.GetCommentForUpdate(ctx, *c.ThreadID)
	if err != nil {
		return fmt.Errorf("leer hilo: %w", err)
	}
	if root.DeletedAt == nil {
		return nil
	}
	if n, err := q.CountReplies(ctx, &root.ID); err != nil || n > 0 {
		return err
	}
	if err := q.DeleteComment(ctx, root.ID); err != nil {
		return fmt.Errorf("borrar hilo: %w", err)
	}
	return nil
}

// SetResolved resolves or reopens a thread (RF-36, RF-37).
func (s *Service) SetResolved(ctx context.Context, p *authz.Principal, id uuid.UUID, resolved bool) (Result, error) {
	var out Result
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		c, t, err := comment(ctx, q, p, id)
		if err != nil {
			return err
		}
		if c.ThreadID != nil {
			return apperr.ErrValidation.WithDetail("field", "id")
		}
		if (c.ResolvedAt != nil) != resolved {
			if _, err := q.SetThreadResolved(ctx, sqlc.SetThreadResolvedParams{ID: id, Resolved: resolved, ResolvedBy: &p.UserID}); err != nil {
				return fmt.Errorf("resolver hilo: %w", err)
			}
			action := "reopened"
			if resolved {
				action = "resolved"
			}
			if err := record(ctx, q, p, t, action, id, nil); err != nil {
				return err
			}
			if !t.Project.IsTemplate {
				typ := events.CommentThreadReopened
				if resolved {
					typ = events.CommentThreadResolved
				}
				actor := p.UserID
				if err := events.Emit(ctx, q, events.New{Type: typ, ActorID: &actor, ProjectID: t.Project.ID,
					SubjectType: t.Kind, SubjectID: t.ID, Payload: map[string]any{"thread_id": id}}); err != nil {
					return err
				}
			}
		}
		out.Comments, err = list(ctx, q, p, t)
		return err
	})
	return out, err
}

// Upload stores a file for the comments of a work item or a meeting (RF-26).
// It is not tied to a comment: the comment that uses it may not exist yet.
func (s *Service) Upload(ctx context.Context, p *authz.Principal, kind string, id uuid.UUID, name string, data []byte) (files.File, error) {
	var out files.File
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		t, err := load(ctx, q, p, kind, id, false)
		if err != nil {
			return err
		}
		if err := mayComment(p, t.Project); err != nil {
			return err
		}
		out, err = s.files.Upload(ctx, q, t.filesOwner(), p.UserID, name, data)
		return err
	})
	return out, err
}

// emitCreated records the comment for the notifications (spec 017): its
// thread, who it mentions and the first 140 characters of its text (RF-24).
func emitCreated(ctx context.Context, q *sqlc.Queries, p *authz.Principal, t target, c sqlc.WorkItemComment, text string) error {
	if t.Project.IsTemplate {
		return nil
	}
	thread := c.ID
	if c.ThreadID != nil {
		thread = *c.ThreadID
	}
	var mentioned []uuid.UUID
	for _, m := range richtext.Mentions(c.Body) {
		if m.Kind == "user" && !slices.Contains(mentioned, m.ID) {
			mentioned = append(mentioned, m.ID)
		}
	}
	excerpt := []rune(strings.Join(strings.Fields(text), " "))
	if len(excerpt) > 140 {
		excerpt = append(excerpt[:139], '…')
	}
	actor := p.UserID
	return events.Emit(ctx, q, events.New{
		Type: events.CommentCreated, ActorID: &actor, ProjectID: t.Project.ID, SubjectType: t.Kind, SubjectID: t.ID,
		Payload: map[string]any{"comment_id": c.ID, "thread_id": thread, "reply": c.ThreadID != nil, "mentions": mentioned, "excerpt": string(excerpt)},
	})
}
