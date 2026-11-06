package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"omni-cd/internal/maintenance"
	"omni-cd/internal/state"
)

func maintenanceServer(t *testing.T) *Server {
	t.Helper()
	st := state.New(100, "", true, nil)
	st.SetClusters([]state.ResourceInfo{{ID: "prod-eu"}, {ID: "prod-us"}})
	return &Server{appState: st}
}

func doMW(t *testing.T, s *Server, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	s.handleMaintenanceWindows(rec, httptest.NewRequest(method, "/api/maintenance-windows", strings.NewReader(body)))
	return rec
}

func TestHandleMaintenanceWindows_Lifecycle(t *testing.T) {
	s := maintenanceServer(t)

	rec := doMW(t, s, http.MethodPost, `{"name":"Weekend","clusters":["prod-eu","prod-us"],"days":["sat"],"start":"06:00","end":"10:00"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var created maintenance.Window
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil || created.ID == "" {
		t.Fatalf("create response %s: %v", rec.Body, err)
	}

	update := `{"id":"` + created.ID + `","name":"Weekend EU","clusters":["prod-eu"],"days":["sun"],"start":"22:00","end":"02:00"}`
	if rec := doMW(t, s, http.MethodPut, update); rec.Code != http.StatusOK {
		t.Fatalf("update: %d %s", rec.Code, rec.Body)
	}
	got := s.appState.GetMaintenanceWindows()
	if len(got) != 1 || got[0].Name != "Weekend EU" || len(got[0].Clusters) != 1 || got[0].Days[0] != "sun" {
		t.Fatalf("after update: %+v", got)
	}

	if rec := doMW(t, s, http.MethodDelete, `{"id":"`+created.ID+`"}`); rec.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body)
	}
	if n := len(s.appState.GetMaintenanceWindows()); n != 0 {
		t.Fatalf("%d windows left after delete", n)
	}

	actions := map[string]bool{}
	for _, e := range s.appState.GetAuditLog() {
		actions[e.Action] = true
	}
	for _, a := range []string{"maintenance-window-create", "maintenance-window-update", "maintenance-window-delete"} {
		if !actions[a] {
			t.Errorf("audit log missing %q", a)
		}
	}
}

func TestHandleMaintenanceWindows_Errors(t *testing.T) {
	s := maintenanceServer(t)
	cases := []struct {
		name, method, body string
		want               int
	}{
		{"GET not allowed", http.MethodGet, "", http.StatusMethodNotAllowed},
		{"bad json", http.MethodPost, `{`, http.StatusBadRequest},
		{"unknown cluster", http.MethodPost, `{"name":"w","clusters":["nope"],"days":["sat"],"start":"06:00","end":"10:00"}`, http.StatusBadRequest},
		{"invalid range", http.MethodPost, `{"name":"w","clusters":["prod-eu"],"days":["sat"],"start":"06:00","end":"06:00"}`, http.StatusBadRequest},
		{"put without id", http.MethodPut, `{"name":"w"}`, http.StatusBadRequest},
		{"put unknown id", http.MethodPut, `{"id":"nope","name":"w","clusters":["prod-eu"],"days":["sat"],"start":"06:00","end":"10:00"}`, http.StatusNotFound},
		{"delete unknown id", http.MethodDelete, `{"id":"nope"}`, http.StatusNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if rec := doMW(t, s, c.method, c.body); rec.Code != c.want {
				t.Errorf("%s: got %d (%s), want %d", c.name, rec.Code, strings.TrimSpace(rec.Body.String()), c.want)
			}
		})
	}
}

func TestHandleMaintenanceWindows_SelectorOnly(t *testing.T) {
	s := maintenanceServer(t)
	rec := doMW(t, s, http.MethodPost, `{"name":"Prod","selector":{"env":"production"},"days":["sat"],"start":"06:00","end":"10:00"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create with selector only: %d %s", rec.Code, rec.Body)
	}
	got := s.appState.GetMaintenanceWindows()
	if len(got) != 1 || got[0].Selector["env"] != "production" || len(got[0].Clusters) != 0 {
		t.Fatalf("stored window = %+v", got)
	}
	rec = doMW(t, s, http.MethodPost, `{"name":"Bad","selector":{"omni.sidero.dev/cluster":"x"},"days":["sat"],"start":"06:00","end":"10:00"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("system label selector: got %d, want 400", rec.Code)
	}
}
