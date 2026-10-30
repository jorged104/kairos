// Package comments keeps the comments of work items (spec 006, RF-22 to
// RF-55): threads with one level of replies that can be resolved, and
// threads anchored to a piece of the work item's page.
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
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"openscum/internal/apperr"
	"openscum/internal/audit"
	"openscum/internal/authz"
	"openscum/internal/db"
	"openscum/internal/db/sqlc"
	"openscum/internal/files"
	"openscum/internal/mentions"
	"openscum/internal/projects"
	"openscum/internal/richtext"
	"openscum/internal/workcore"
	"openscum/internal/workitems"
)

// OwnerKind owns the files of the comments of a work item, by its id.
const OwnerKind = "work_item_comment"

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
	Can       Can             `json:"can"`
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

// List is every thread of a work item, oldest first (RF-29), with the
// mentions of all of them.
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

// item loads the work item, which also checks the caller can read it.
func item(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID, lock bool) (sqlc.WorkItem, sqlc.Project, error) {
	return workitems.Load(ctx, q, p, id, lock)
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

func owner(w sqlc.WorkItem) files.Owner {
	return files.Owner{Kind: OwnerKind, ID: w.ID, ProjectID: w.ProjectID}
}

// body validates the body of a comment against the files of its work item.
func body(ctx context.Context, q *sqlc.Queries, w sqlc.WorkItem, raw json.RawMessage) (json.RawMessage, string, error) {
	owned, err := files.Owned(ctx, q, owner(w))
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

func record(ctx context.Context, q *sqlc.Queries, p *authz.Principal, action string, id uuid.UUID, after map[string]any) error {
	// Never the text (RF-42).
	return audit.Record(ctx, q, audit.Entry{Actor: audit.ByUser(p), Action: "work_item.comment_" + action,
		TargetType: "work_item_comment", TargetID: id.String(), After: after})
}

// List returns the threads of a work item.
func (s *Service) List(ctx context.Context, p *authz.Principal, workItemID uuid.UUID) (List, error) {
	w, project, err := item(ctx, s.store.Queries, p, workItemID, false)
	if err != nil {
		return List{}, err
	}
	return list(ctx, s.store.Queries, p, w, project)
}

func list(ctx context.Context, q *sqlc.Queries, p *authz.Principal, w sqlc.WorkItem, project sqlc.Project) (List, error) {
	rows, err := q.ListComments(ctx, w.ID)
	if err != nil {
		return List{}, fmt.Errorf("leer comentarios: %w", err)
	}
	can := canComment(p, project)
	manager := p.CanManageProject(projects.Ref(project))
	live := richtext.Threads(w.Description)
	out := List{Threads: []Thread{}, CanComment: can}
	at := map[uuid.UUID]int{}
	var bodies [][]byte
	for _, r := range rows {
		mine := r.AuthorID == p.UserID
		deleted := r.DeletedAt != nil
		c := Comment{
			ID: r.ID, Author: workcore.Ref{ID: r.AuthorID, Name: r.AuthorName}, Body: r.Body,
			CreatedAt: r.CreatedAt, EditedAt: r.EditedAt, Deleted: deleted,
			Can: Can{Edit: can && mine && !deleted, Delete: can && (mine || manager) && !deleted},
		}
		if len(r.Body) > 0 {
			bodies = append(bodies, r.Body)
		}
		if r.ThreadID == nil {
			t := Thread{Comment: c, Quote: r.Quote, Anchored: r.Anchored, AnchorDeleted: r.Anchored && !live[r.ID],
				ResolvedAt: r.ResolvedAt, Replies: []Comment{}}
			if r.ResolvedBy != nil && r.ResolvedByName != nil {
				t.ResolvedBy = &workcore.Ref{ID: *r.ResolvedBy, Name: *r.ResolvedByName}
			}
			at[r.ID] = len(out.Threads)
			out.Threads = append(out.Threads, t)
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
func (s *Service) Create(ctx context.Context, p *authz.Principal, workItemID uuid.UUID, req createRequest) (Result, error) {
	var out Result
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		w, project, err := item(ctx, q, p, workItemID, true)
		if err != nil {
			return err
		}
		if err := mayComment(p, project); err != nil {
			return err
		}
		doc, text, err := body(ctx, q, w, req.Body)
		if err != nil {
			return err
		}
		params := sqlc.CreateCommentParams{WorkItemID: w.ID, AuthorID: p.UserID, Body: doc, BodyText: text}
		after := map[string]any{"work_item_id": w.ID}
		switch {
		case req.ThreadID != nil && req.Anchor != nil:
			return apperr.ErrValidation.WithDetail("field", "anchor")
		case req.ThreadID != nil:
			root, err := thread(ctx, q, w, *req.ThreadID)
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
			saved, err := anchor(ctx, q, p, w, *req.Anchor)
			if err != nil {
				return err
			}
			out.Page = &saved
			params.ID, params.Quote, params.Anchored = &req.Anchor.ThreadID, &req.Anchor.Quote, true
			after["anchored"] = true
			if w, err = q.GetWorkItem(ctx, w.ID); err != nil {
				return fmt.Errorf("releer work item: %w", err)
			}
		}
		c, err := q.CreateComment(ctx, params)
		if err != nil {
			return fmt.Errorf("crear comentario: %w", err)
		}
		if err := record(ctx, q, p, "created", c.ID, after); err != nil {
			return err
		}
		out.Comments, err = list(ctx, q, p, w, project)
		return err
	})
	return out, err
}

// thread returns the root of the thread of comment id, in work item w.
func thread(ctx context.Context, q *sqlc.Queries, w sqlc.WorkItem, id uuid.UUID) (sqlc.WorkItemComment, error) {
	c, err := q.GetCommentForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && c.WorkItemID != w.ID) {
		return c, apperr.ErrNotFound
	}
	if err != nil {
		return c, fmt.Errorf("leer comentario: %w", err)
	}
	if c.ThreadID != nil {
		return thread(ctx, q, w, *c.ThreadID)
	}
	return c, nil
}

// anchor saves the page with the new thread's mark. The page must be the
// stored one, with the same anchors plus the new one: whoever can't edit the
// page only marks it (RF-46).
func anchor(ctx context.Context, q *sqlc.Queries, p *authz.Principal, w sqlc.WorkItem, a anchorRequest) (PageSaved, error) {
	if a.PageVersion != w.PageVersion {
		return PageSaved{}, pageConflict(w.PageVersion)
	}
	exists, err := q.CommentExists(ctx, a.ThreadID)
	if err != nil {
		return PageSaved{}, fmt.Errorf("comprobar hilo: %w", err)
	}
	if exists {
		return PageSaved{}, apperr.ErrValidation.WithDetail("field", "thread_id")
	}
	doc, text, page, err := files.Page(ctx, q, files.Owner{Kind: "work_item", ID: w.ID, ProjectID: w.ProjectID}, a.Page)
	if err != nil {
		return PageSaved{}, err
	}
	before := richtext.Threads(w.Description)
	if !slices.Contains(page.Threads, a.ThreadID) || len(page.Threads) != len(before)+1 {
		return PageSaved{}, errPageChanged
	}
	for _, id := range page.Threads {
		if id != a.ThreadID && !before[id] {
			return PageSaved{}, errPageChanged
		}
	}
	if !richtext.SameButAnchors(doc, w.Description) {
		return PageSaved{}, errPageChanged
	}
	saved, err := q.UpdateWorkItemPage(ctx, sqlc.UpdateWorkItemPageParams{ID: w.ID, PageVersion: a.PageVersion, Description: doc, DescriptionText: text})
	if err != nil {
		return PageSaved{}, fmt.Errorf("guardar página: %w", err)
	}
	refs, err := mentions.Refs(ctx, q, p, saved.Description)
	if err != nil {
		return PageSaved{}, err
	}
	return PageSaved{Page: saved.Description, PageVersion: saved.PageVersion, Version: saved.Version, Refs: refs}, nil
}

// comment loads a comment with its work item, locked, for a change.
func comment(ctx context.Context, q *sqlc.Queries, p *authz.Principal, id uuid.UUID) (sqlc.WorkItemComment, sqlc.WorkItem, sqlc.Project, error) {
	c, err := q.GetCommentForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, sqlc.WorkItem{}, sqlc.Project{}, apperr.ErrNotFound
	}
	if err != nil {
		return c, sqlc.WorkItem{}, sqlc.Project{}, fmt.Errorf("leer comentario: %w", err)
	}
	w, project, err := item(ctx, q, p, c.WorkItemID, false)
	if err != nil {
		return c, w, project, err
	}
	if err := mayComment(p, project); err != nil {
		return c, w, project, err
	}
	return c, w, project, nil
}

// Update changes the text of one's own comment (RF-31, RF-35).
func (s *Service) Update(ctx context.Context, p *authz.Principal, id uuid.UUID, req updateRequest) (Result, error) {
	var out Result
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		c, w, project, err := comment(ctx, q, p, id)
		if err != nil {
			return err
		}
		if c.DeletedAt != nil {
			return apperr.ErrNotFound
		}
		if c.AuthorID != p.UserID {
			return apperr.ErrForbidden
		}
		doc, text, err := body(ctx, q, w, req.Body)
		if err != nil {
			return err
		}
		if _, err := q.UpdateCommentBody(ctx, sqlc.UpdateCommentBodyParams{ID: id, Body: doc, BodyText: text}); err != nil {
			return fmt.Errorf("editar comentario: %w", err)
		}
		if err := record(ctx, q, p, "updated", id, map[string]any{"work_item_id": w.ID}); err != nil {
			return err
		}
		out.Comments, err = list(ctx, q, p, w, project)
		return err
	})
	return out, err
}

// Delete removes a comment: its author, or whoever manages the project
// (RF-32 to RF-34). A root with replies stays as «Comentario eliminado».
func (s *Service) Delete(ctx context.Context, p *authz.Principal, id uuid.UUID) (Result, error) {
	var out Result
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		c, w, project, err := comment(ctx, q, p, id)
		if err != nil {
			return err
		}
		if c.DeletedAt != nil {
			return apperr.ErrNotFound
		}
		if c.AuthorID != p.UserID && !p.CanManageProject(projects.Ref(project)) {
			return apperr.ErrForbidden
		}
		if err := remove(ctx, q, c); err != nil {
			return err
		}
		if err := record(ctx, q, p, "deleted", id, map[string]any{"work_item_id": w.ID, "author_id": c.AuthorID}); err != nil {
			return err
		}
		out.Comments, err = list(ctx, q, p, w, project)
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
		c, w, project, err := comment(ctx, q, p, id)
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
			if err := record(ctx, q, p, action, id, map[string]any{"work_item_id": w.ID}); err != nil {
				return err
			}
		}
		out.Comments, err = list(ctx, q, p, w, project)
		return err
	})
	return out, err
}

// Upload stores a file for the comments of a work item (RF-26). It is not
// tied to a comment: the comment that uses it may not exist yet.
func (s *Service) Upload(ctx context.Context, p *authz.Principal, workItemID uuid.UUID, name string, data []byte) (files.File, error) {
	var out files.File
	err := s.store.InTx(ctx, func(_ pgx.Tx, q *sqlc.Queries) error {
		w, project, err := item(ctx, q, p, workItemID, false)
		if err != nil {
			return err
		}
		if err := mayComment(p, project); err != nil {
			return err
		}
		out, err = s.files.Upload(ctx, q, owner(w), p.UserID, name, data)
		return err
	})
	return out, err
}
