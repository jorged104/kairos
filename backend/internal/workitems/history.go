package workitems

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/google/uuid"

	"openscum/internal/authz"
	"openscum/internal/db/sqlc"
	"openscum/internal/workcore"
)

// History is a page of the activity of a work item, newest first (spec 007,
// RF-47 to RF-54). Next is the cursor of the following page.
type History struct {
	Entries []HistoryEntry `json:"entries"`
	Next    *int64         `json:"next"`
}

// HistoryEntry is one change: created, updated, description (the page only,
// RF-50), dependency_added or dependency_removed.
type HistoryEntry struct {
	At      time.Time    `json:"at"`
	Actor   workcore.Ref `json:"actor"`
	Kind    string       `json:"kind"`
	Changes []Change     `json:"changes"`
	// The oldest audit row of the entry, when several are joined (RF-51).
	oldest   int64
	oldestAt time.Time
}

// Change of one property. Ids come as {id, name} with their visible name
// (RF-49), with an empty name when it no longer exists; the description has
// no values (RF-50).
type Change struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

const (
	historyPage       = 50
	historyBatch      = 100
	descriptionWindow = 10 * time.Minute // RF-51
)

// historyFields are compared in this order; the kind tells how to name their values.
var historyFields = []struct{ name, kind string }{
	{"title", ""}, {"type_id", "type"}, {"status_id", "status"}, {"priority_id", "priority"},
	{"parent_id", "work_item"}, {"assignee_ids", "users"}, {"start_date", ""}, {"due_date", ""},
	{"estimated_hours", ""}, {"remaining_hours", ""}, {"done_ratio", ""}, {"weight", ""},
	{"size_id", "size"}, {"label_ids", "labels"},
}

func (s *Service) History(ctx context.Context, p *authz.Principal, id uuid.UUID, before *int64) (History, error) {
	q := s.store.Queries
	if _, _, err := Load(ctx, q, p, id, false); err != nil {
		return History{}, err
	}
	out := History{Entries: []HistoryEntry{}}
	cursor := before
	var entries []HistoryEntry
	for len(entries) <= historyPage {
		rows, err := q.WorkItemHistoryPage(ctx, sqlc.WorkItemHistoryPageParams{TargetID: id.String(), BeforeID: cursor, RowLimit: historyBatch})
		if err != nil {
			return out, fmt.Errorf("leer historial: %w", err)
		}
		for _, r := range rows {
			e, ok := historyEntry(r)
			if !ok {
				continue
			}
			if n := len(entries); n > 0 && mergeable(entries[n-1], e) {
				entries[n-1].oldest, entries[n-1].oldestAt = e.oldest, e.oldestAt
				continue
			}
			entries = append(entries, e)
		}
		if len(rows) < historyBatch {
			break
		}
		last := rows[len(rows)-1].ID
		cursor = &last
	}
	if len(entries) > historyPage {
		entries = entries[:historyPage]
		next := entries[historyPage-1].oldest
		out.Next = &next
	}
	if err := nameValues(ctx, q, entries); err != nil {
		return out, err
	}
	out.Entries = append(out.Entries, entries...)
	return out, nil
}

// mergeable joins edits of the description by one person less than 10 minutes apart (RF-51).
func mergeable(newer, older HistoryEntry) bool {
	return newer.Kind == "description" && older.Kind == "description" && newer.Actor.ID == older.Actor.ID &&
		newer.oldestAt.Sub(older.At) < descriptionWindow
}

func historyEntry(r sqlc.WorkItemHistoryPageRow) (HistoryEntry, bool) {
	e := HistoryEntry{At: r.OccurredAt, Actor: workcore.Ref{Name: r.ActorName}, Changes: []Change{}, oldest: r.ID, oldestAt: r.OccurredAt}
	if r.ActorID != nil {
		e.Actor.ID = *r.ActorID
	}
	before, after := decode(r.Before), decode(r.After)
	switch r.Action {
	case "work_item.created":
		e.Kind = "created"
	case "work_item.page_updated":
		e.Kind = "description"
		e.Changes = append(e.Changes, Change{Field: "description"})
	case "work_item.dependency_added", "work_item.dependency_removed":
		e.Kind = "dependency_added"
		dep := after
		if r.Action == "work_item.dependency_removed" {
			e.Kind, dep = "dependency_removed", before
		}
		e.Changes = append(e.Changes, Change{Field: fmt.Sprint(dep["type"]), To: ref{"work_item", dep["to_id"]}})
	default:
		e.Kind = "updated"
		// Before spec 006 the audit kept only one assignee.
		if _, ok := after["assignee_ids"]; !ok {
			before["assignee_ids"], after["assignee_ids"] = listOf(before["assignee_id"]), listOf(after["assignee_id"])
		}
		for _, f := range historyFields {
			from, ok1 := before[f.name]
			to, ok2 := after[f.name]
			if !ok1 || !ok2 || reflect.DeepEqual(from, to) {
				continue
			}
			if f.kind != "" {
				from, to = ref{f.kind, from}, ref{f.kind, to}
			}
			e.Changes = append(e.Changes, Change{Field: f.name, From: from, To: to})
		}
		if changed, _ := after["description_changed"].(bool); changed {
			e.Changes = append(e.Changes, Change{Field: "description"})
			if len(e.Changes) == 1 {
				e.Kind = "description"
			}
		}
		if len(e.Changes) == 0 {
			return e, false
		}
	}
	return e, true
}

func decode(raw json.RawMessage) map[string]any {
	out := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func listOf(v any) []any {
	if v == nil {
		return []any{}
	}
	return []any{v}
}

// ref is a value to name before answering.
type ref struct {
	kind  string
	value any
}

type named struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// nameValues replaces the refs of the entries by their names, one query per kind.
func nameValues(ctx context.Context, q *sqlc.Queries, entries []HistoryEntry) error {
	names := map[string]map[string]string{}
	wanted := map[string][]string{}
	each := func(fn func(c *any)) {
		for i := range entries {
			for j := range entries[i].Changes {
				fn(&entries[i].Changes[j].From)
				fn(&entries[i].Changes[j].To)
			}
		}
	}
	each(func(c *any) {
		r, ok := (*c).(ref)
		if !ok {
			return
		}
		kind := r.kind
		if kind == "users" || kind == "labels" {
			kind = kind[:len(kind)-1]
			if list, ok := r.value.([]any); ok {
				for _, v := range list {
					wanted[kind] = append(wanted[kind], fmt.Sprint(v))
				}
			}
			return
		}
		if r.value != nil {
			wanted[kind] = append(wanted[kind], fmt.Sprint(r.value))
		}
	})
	for kind, ids := range wanted {
		m, err := lookup(ctx, q, kind, ids)
		if err != nil {
			return err
		}
		names[kind] = m
	}
	each(func(c *any) {
		r, ok := (*c).(ref)
		if !ok {
			return
		}
		one := func(kind string, v any) any {
			if v == nil {
				return nil
			}
			id := fmt.Sprint(v)
			return named{ID: id, Name: names[kind][id]}
		}
		switch r.kind {
		case "users", "labels":
			kind := r.kind[:len(r.kind)-1]
			list := []any{}
			if vs, ok := r.value.([]any); ok {
				for _, v := range vs {
					list = append(list, one(kind, v))
				}
			}
			*c = list
		default:
			*c = one(r.kind, r.value)
		}
	})
	return nil
}

func lookup(ctx context.Context, q *sqlc.Queries, kind string, raw []string) (map[string]string, error) {
	out := map[string]string{}
	ids := make([]uuid.UUID, 0, len(raw))
	for _, v := range raw {
		if id, err := uuid.Parse(v); err == nil {
			ids = append(ids, id)
		}
	}
	add := func(id uuid.UUID, name string) { out[id.String()] = name }
	switch kind {
	case "user":
		rows, err := q.NamesOfUsers(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("leer personas: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Name)
		}
	case "work_item":
		rows, err := q.WorkItemRefs(ctx, ids)
		if err != nil {
			return nil, fmt.Errorf("leer referencias: %w", err)
		}
		for _, r := range rows {
			add(r.ID, Key(r.Identifier, r.SequenceNumber)+" "+r.Title)
		}
	case "status":
		rows, err := q.ListStatuses(ctx)
		if err != nil {
			return nil, fmt.Errorf("leer estados: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Name)
		}
	case "type":
		rows, err := q.ListWorkItemTypes(ctx)
		if err != nil {
			return nil, fmt.Errorf("leer tipos: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Name)
		}
	case "priority":
		rows, err := q.ListPriorities(ctx)
		if err != nil {
			return nil, fmt.Errorf("leer prioridades: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Name)
		}
	case "size":
		rows, err := q.ListWorkItemSizes(ctx)
		if err != nil {
			return nil, fmt.Errorf("leer tallas: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Name)
		}
	case "label":
		rows, err := q.ListLabels(ctx)
		if err != nil {
			return nil, fmt.Errorf("leer etiquetas: %w", err)
		}
		for _, r := range rows {
			add(r.ID, r.Name)
		}
	}
	return out, nil
}
