package apitest

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestPagePerformance checks the autosave of the largest page of spec 005 (50 000
// characters and 20 images) against its limit of 300 ms (median of 5).
func TestPagePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("prueba de rendimiento omitida con -short")
	}
	s := newWorkSetup(t)
	wi := s.item(t, s.member, s.workProject(t), map[string]any{})
	owner := "work-items/" + wi.ID.String()

	blocks := make([]string, 0, 120)
	// 99 paragraphs of 500 characters plus their mentions: 49 500 characters.
	for i := 0; i < 99; i++ {
		blocks = append(blocks, `{"type":"paragraph","content":[{"type":"text","text":"`+strings.Repeat("a", 500)+`"},`+
			`{"type":"mention","attrs":{"kind":"user","id":"`+s.leadID.String()+`"}}]}`)
	}
	for i := 0; i < 20; i++ {
		f := s.upload(t, s.member, owner, "captura.png", pngData, http.StatusCreated)
		blocks = append(blocks, `{"type":"image","attrs":{"fileId":"`+f.ID.String()+`"}}`)
	}
	doc := `{"type":"doc","content":[` + strings.Join(blocks, ",") + `]}`

	version := int32(1)
	times := make([]time.Duration, 0, 5)
	for i := 0; i < 5; i++ {
		// Each save changes one character, as the autosave of a user typing does.
		page := strings.Replace(doc, "aaaa", "aaa"+string(rune('b'+i)), 1)
		start := time.Now()
		res := s.member.Do(http.MethodPut, "/api/"+owner+"/page", map[string]any{"page": json.RawMessage(page), "page_version": version}).
			Expect(t, http.StatusOK)
		times = append(times, time.Since(start))
		var saved savedPageDTO
		res.JSON(t, &saved)
		version = saved.PageVersion
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	t.Logf("autoguardado de página grande: mediana %v (límite 300ms)", times[2].Round(time.Millisecond))
	if times[2] > 300*time.Millisecond {
		t.Errorf("autoguardado: mediana %v supera 300ms", times[2])
	}
}
