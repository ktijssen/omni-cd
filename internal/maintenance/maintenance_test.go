package maintenance

import (
	"strings"
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func r(start, end string, days ...string) TimeRange {
	return TimeRange{Days: days, Start: start, End: end}
}

func TestWindowValidate(t *testing.T) {
	ok := r("22:00", "02:00", "tue")
	cases := []struct {
		name    string
		w       Window
		wantErr string
	}{
		{"valid", Window{Name: "prod", Clusters: []string{"a", "b"}, TimeRange: ok}, ""},
		{"no name", Window{Name: "  ", Clusters: []string{"a"}, TimeRange: ok}, "name is required"},
		{"long name", Window{Name: strings.Repeat("x", 65), Clusters: []string{"a"}, TimeRange: ok}, "at most 64"},
		{"no clusters", Window{Name: "w", TimeRange: ok}, "at least one cluster"},
		{"empty cluster", Window{Name: "w", Clusters: []string{""}, TimeRange: ok}, "must not be empty"},
		{"dup cluster", Window{Name: "w", Clusters: []string{"a", "a"}, TimeRange: ok}, "listed twice"},
		{"no range", Window{Name: "w", Clusters: []string{"a"}}, "at least one day"},
		{"no days", Window{Name: "w", Clusters: []string{"a"}, TimeRange: r("01:00", "02:00")}, "at least one day"},
		{"bad day", Window{Name: "w", Clusters: []string{"a"}, TimeRange: r("01:00", "02:00", "tuesday")}, "unknown day"},
		{"dup day", Window{Name: "w", Clusters: []string{"a"}, TimeRange: r("01:00", "02:00", "mon", "mon")}, "listed twice"},
		{"bad start", Window{Name: "w", Clusters: []string{"a"}, TimeRange: r("25:00", "02:00", "mon")}, "start"},
		{"short start", Window{Name: "w", Clusters: []string{"a"}, TimeRange: r("1:00", "02:00", "mon")}, "start"},
		{"equal", Window{Name: "w", Clusters: []string{"a"}, TimeRange: r("02:00", "02:00", "mon")}, "must differ"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.w.Validate()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("Validate() = %v, want error containing %q", err, c.wantErr)
			}
		})
	}
}

func TestEvaluate_SameDayRange(t *testing.T) {
	ams := mustLoc(t, "Europe/Amsterdam")
	ranges := []TimeRange{r("06:00", "10:00", "sat")}

	// 2026-11-07 is a Saturday.
	cases := []struct {
		at       time.Time
		open     bool
		end      time.Time
		next     time.Time
		describe string
	}{
		{time.Date(2026, 11, 7, 5, 59, 0, 0, ams), false, time.Time{}, time.Date(2026, 11, 7, 6, 0, 0, 0, ams), "just before"},
		{time.Date(2026, 11, 7, 6, 0, 0, 0, ams), true, time.Date(2026, 11, 7, 10, 0, 0, 0, ams), time.Date(2026, 11, 14, 6, 0, 0, 0, ams), "start is inclusive"},
		{time.Date(2026, 11, 7, 9, 59, 0, 0, ams), true, time.Date(2026, 11, 7, 10, 0, 0, 0, ams), time.Date(2026, 11, 14, 6, 0, 0, 0, ams), "inside"},
		{time.Date(2026, 11, 7, 10, 0, 0, 0, ams), false, time.Time{}, time.Date(2026, 11, 14, 6, 0, 0, 0, ams), "end is exclusive"},
		{time.Date(2026, 11, 4, 12, 0, 0, 0, ams), false, time.Time{}, time.Date(2026, 11, 7, 6, 0, 0, 0, ams), "midweek"},
	}
	for _, c := range cases {
		t.Run(c.describe, func(t *testing.T) {
			st := Evaluate(ranges, c.at, ams)
			if st.Open != c.open || !st.CurrentEnd.Equal(c.end) || !st.NextStart.Equal(c.next) {
				t.Errorf("Evaluate(%v) = %+v; want open=%v end=%v next=%v", c.at, st, c.open, c.end, c.next)
			}
		})
	}
}

func TestEvaluate_CrossesMidnight(t *testing.T) {
	ams := mustLoc(t, "Europe/Amsterdam")
	ranges := []TimeRange{r("22:00", "02:00", "tue", "thu")}
	open := func(at time.Time) bool { return Evaluate(ranges, at, ams).Open }

	// 2026-11-03 is a Tuesday.
	if !open(time.Date(2026, 11, 3, 23, 30, 0, 0, ams)) {
		t.Error("Tue 23:30 should be inside")
	}
	if !open(time.Date(2026, 11, 4, 1, 59, 0, 0, ams)) {
		t.Error("Wed 01:59 should be inside the range that started Tuesday")
	}
	if open(time.Date(2026, 11, 4, 2, 0, 0, 0, ams)) {
		t.Error("Wed 02:00 should be outside")
	}
	if open(time.Date(2026, 11, 3, 1, 0, 0, 0, ams)) {
		t.Error("Tue 01:00 should be outside (Monday has no range)")
	}
	st := Evaluate(ranges, time.Date(2026, 11, 4, 1, 0, 0, 0, ams), ams)
	if want := time.Date(2026, 11, 5, 22, 0, 0, 0, ams); !st.NextStart.Equal(want) {
		t.Errorf("NextStart = %v, want Thursday 22:00 (%v)", st.NextStart, want)
	}
}

func TestEvaluate_EndAtMidnight(t *testing.T) {
	ranges := []TimeRange{r("20:00", "00:00", "mon")}
	// 2026-11-02 is a Monday.
	if !Evaluate(ranges, time.Date(2026, 11, 2, 23, 59, 0, 0, time.UTC), time.UTC).Open {
		t.Error("Mon 23:59 should be inside")
	}
	if Evaluate(ranges, time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), time.UTC).Open {
		t.Error("Tue 00:00 should be outside")
	}
}

func TestEvaluate_LocationIsRespected(t *testing.T) {
	// Tue 22:00 in New York is Wed 03:00 UTC (EST, UTC-5).
	ny := mustLoc(t, "America/New_York")
	ranges := []TimeRange{r("22:00", "23:00", "tue")}
	if !Evaluate(ranges, time.Date(2026, 11, 4, 3, 30, 0, 0, time.UTC), ny).Open {
		t.Error("Wed 03:30 UTC is Tue 22:30 in New York and should be inside")
	}
	if Evaluate(ranges, time.Date(2026, 11, 4, 3, 30, 0, 0, time.UTC), time.UTC).Open {
		t.Error("the same instant evaluated in UTC is Wed 03:30 and should be outside")
	}
	if !Evaluate(ranges, time.Date(2026, 11, 3, 22, 30, 0, 0, time.UTC), nil).Open {
		t.Error("a nil location should mean UTC")
	}
}

func TestEvaluate_DSTTransition(t *testing.T) {
	ams := mustLoc(t, "Europe/Amsterdam")
	// Europe switches to CEST on 2026-03-29 (Sunday) at 02:00 → 03:00.
	ranges := []TimeRange{r("01:00", "04:00", "sun")}
	st := Evaluate(ranges, time.Date(2026, 3, 29, 3, 30, 0, 0, ams), ams)
	if !st.Open {
		t.Fatalf("03:30 CEST on DST day should be inside: %+v", st)
	}
	if want := time.Date(2026, 3, 29, 4, 0, 0, 0, ams); !st.CurrentEnd.Equal(want) {
		t.Errorf("CurrentEnd = %v, want %v", st.CurrentEnd, want)
	}
}

func TestEvaluate_AdjacentRangesMerge(t *testing.T) {
	ranges := []TimeRange{r("22:00", "00:00", "mon"), r("00:00", "02:00", "tue")}
	st := Evaluate(ranges, time.Date(2026, 11, 2, 23, 0, 0, 0, time.UTC), time.UTC) // Monday 23:00
	if want := time.Date(2026, 11, 3, 2, 0, 0, 0, time.UTC); !st.Open || !st.CurrentEnd.Equal(want) {
		t.Errorf("Evaluate = %+v; want open until %v", st, want)
	}
	if want := time.Date(2026, 11, 9, 22, 0, 0, 0, time.UTC); !st.NextStart.Equal(want) {
		t.Errorf("NextStart = %v; want next Monday 22:00 (%v)", st.NextStart, want)
	}
}

func TestEvaluate_InvalidRangesNeverOpen(t *testing.T) {
	ranges := []TimeRange{r("00:00", "00:00", "mon", "tue", "wed", "thu", "fri", "sat", "sun"), r("bogus", "02:00", "mon")}
	if st := Evaluate(ranges, time.Now(), time.UTC); st.Open || !st.NextStart.IsZero() {
		t.Errorf("invalid ranges should never be open: %+v", st)
	}
}

func TestHasCluster(t *testing.T) {
	w := Window{Clusters: []string{"a", "b"}}
	if !w.HasCluster("b") || w.HasCluster("c") {
		t.Error("HasCluster mismatch")
	}
}

func TestWindowValidate_Selector(t *testing.T) {
	ok := r("22:00", "02:00", "tue")
	cases := []struct {
		name    string
		w       Window
		wantErr string
	}{
		{"selector only", Window{Name: "w", TimeRange: ok, Selector: map[string]string{"env": "prod"}}, ""},
		{"selector and clusters", Window{Name: "w", TimeRange: ok, Clusters: []string{"a"}, Selector: map[string]string{"env": "prod"}}, ""},
		{"key-only label", Window{Name: "w", TimeRange: ok, Selector: map[string]string{"backup": ""}}, ""},
		{"neither", Window{Name: "w", TimeRange: ok}, "at least one cluster or label"},
		{"empty key", Window{Name: "w", TimeRange: ok, Selector: map[string]string{"": "x"}}, "must not be empty"},
		{"spaced key", Window{Name: "w", TimeRange: ok, Selector: map[string]string{" env": "x"}}, "surrounding spaces"},
		{"system label", Window{Name: "w", TimeRange: ok, Selector: map[string]string{"omni.sidero.dev/cluster": "a"}}, "managed by Omni"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.w.Validate()
			if c.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("Validate() = %v, want error containing %q", err, c.wantErr)
			}
		})
	}
}

func TestWindowMatches(t *testing.T) {
	w := Window{Clusters: []string{"named"}, Selector: map[string]string{"env": "prod", "region": "eu"}}
	cases := []struct {
		name           string
		id             string
		live, template map[string]string
		want           bool
	}{
		{"listed by name", "named", nil, nil, true},
		{"all labels live", "a", map[string]string{"env": "prod", "region": "eu", "x": "y"}, nil, true},
		{"one label missing (AND)", "a", map[string]string{"env": "prod"}, nil, false},
		{"wrong value", "a", map[string]string{"env": "staging", "region": "eu"}, nil, false},
		{"only in Git template (not yet in Omni)", "a", nil, map[string]string{"env": "prod", "region": "eu"}, true},
		{"live and template combined", "a", map[string]string{"env": "prod"}, map[string]string{"region": "eu"}, true},
		{"no labels", "a", nil, nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := w.Matches(c.id, c.live, c.template); got != c.want {
				t.Errorf("Matches = %v, want %v", got, c.want)
			}
		})
	}
	if (Window{Clusters: []string{"a"}}).Matches("b", map[string]string{"env": "prod"}, nil) {
		t.Error("a window without selector must only match listed clusters")
	}
}
