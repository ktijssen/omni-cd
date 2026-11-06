// Package maintenance models maintenance windows: named sets of recurring
// weekly time ranges during which automatic changes may be applied to the
// clusters the window lists.
//
// Times carry no timezone of their own. They are wall-clock times in the
// location passed to Evaluate — the server's local time (the TZ environment
// variable, UTC when unset).
package maintenance

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	// Embed the IANA timezone database so TZ=Europe/Amsterdam works in the
	// runtime image (alpine), which does not ship tzdata.
	_ "time/tzdata"
)

// TimeRange is one recurring weekly range. Start and End are "HH:MM". When
// End is not after Start the range crosses midnight and ends on the following
// day, so {days: [tue], 22:00–02:00} runs from Tuesday 22:00 until Wednesday
// 02:00. Days name the day the range starts on.
type TimeRange struct {
	Days  []string `json:"days"`  // "mon".."sun"
	Start string   `json:"start"` // "HH:MM"
	End   string   `json:"end"`   // "HH:MM"
}

// Window is a named maintenance window: one weekly time range that applies to
// the listed clusters plus every cluster whose labels match Selector. The
// range's days/start/end are flattened into the window's JSON.
type Window struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	TimeRange
	Clusters []string `json:"clusters"`
	// Selector matches clusters that carry all of these labels (key=value,
	// AND), including clusters created or labelled after the window.
	Selector map[string]string `json:"selector,omitempty"`
}

// Status is a point-in-time evaluation, for display.
type Status struct {
	Open bool `json:"open"`
	// CurrentEnd is when the open window closes (zero when closed).
	CurrentEnd time.Time `json:"currentEnd,omitzero"`
	// NextStart is the start of the next window that is not currently open.
	NextStart time.Time `json:"nextStart,omitzero"`
}

var dayIndex = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// Limits for windows submitted via the API.
const (
	maxNameLen        = 64
	maxClusters       = 1000
	maxSelectorLabels = 16
)

// systemLabelPrefix marks Omni's own labels. They are hidden from cluster
// labels, so a selector on them could never match.
const systemLabelPrefix = "omni.sidero.dev/"

// Validate reports the first problem with w, or nil if it is usable. The ID
// is not checked; the store assigns it.
func (w Window) Validate() error {
	name := strings.TrimSpace(w.Name)
	if name == "" {
		return errors.New("name is required")
	}
	if len(name) > maxNameLen {
		return fmt.Errorf("name must be at most %d characters", maxNameLen)
	}
	if len(w.Clusters) == 0 && len(w.Selector) == 0 {
		return errors.New("select at least one cluster or label")
	}
	if len(w.Selector) > maxSelectorLabels {
		return fmt.Errorf("at most %d labels are allowed", maxSelectorLabels)
	}
	for k := range w.Selector {
		if strings.TrimSpace(k) == "" || k != strings.TrimSpace(k) {
			return errors.New("label keys must not be empty or have surrounding spaces")
		}
		if strings.HasPrefix(k, systemLabelPrefix) {
			return fmt.Errorf("label %q is managed by Omni and cannot be used", k)
		}
	}
	if len(w.Clusters) > maxClusters {
		return fmt.Errorf("at most %d clusters are allowed", maxClusters)
	}
	seen := map[string]bool{}
	for _, c := range w.Clusters {
		if c == "" {
			return errors.New("cluster name must not be empty")
		}
		if seen[c] {
			return fmt.Errorf("cluster %q listed twice", c)
		}
		seen[c] = true
	}
	return w.TimeRange.Validate()
}

// Validate reports the first problem with r, or nil if it is usable.
func (r TimeRange) Validate() error {
	if len(r.Days) == 0 {
		return errors.New("at least one day is required")
	}
	seen := map[string]bool{}
	for _, d := range r.Days {
		if _, ok := dayIndex[d]; !ok {
			return fmt.Errorf("unknown day %q (use mon, tue, wed, thu, fri, sat, sun)", d)
		}
		if seen[d] {
			return fmt.Errorf("day %q listed twice", d)
		}
		seen[d] = true
	}
	start, err := parseClock(r.Start)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	end, err := parseClock(r.End)
	if err != nil {
		return fmt.Errorf("end: %w", err)
	}
	if start == end {
		return errors.New("start and end must differ")
	}
	return nil
}

// Matches reports whether the window applies to a cluster: it is listed by
// name, or its labels satisfy every selector pair. Labels are checked against
// both the live Omni labels and the labels declared in the Git template, so a
// cluster not yet created in Omni, or a label not yet synced, still matches.
func (w Window) Matches(id string, live, template map[string]string) bool {
	if w.HasCluster(id) {
		return true
	}
	if len(w.Selector) == 0 {
		return false
	}
	for k, v := range w.Selector {
		lv, inLive := live[k]
		tv, inTemplate := template[k]
		if !(inLive && lv == v) && !(inTemplate && tv == v) {
			return false
		}
	}
	return true
}

// HasCluster reports whether the window lists the named cluster explicitly.
func (w Window) HasCluster(id string) bool {
	for _, c := range w.Clusters {
		if c == id {
			return true
		}
	}
	return false
}

// Evaluate returns whether any of ranges is open at t (wall-clock times in
// loc), when it closes, and when the next one opens. Overlapping or
// back-to-back ranges count as one, so Mon 22:00–00:00 followed by
// Tue 00:00–02:00 is open until Tuesday 02:00. Invalid ranges never open,
// so a broken configuration cannot let changes through.
func Evaluate(ranges []TimeRange, t time.Time, loc *time.Location) Status {
	var st Status
	for _, iv := range intervals(ranges, t, loc) {
		switch {
		case !t.Before(iv.start) && t.Before(iv.end):
			st.Open = true
			st.CurrentEnd = iv.end
		case iv.start.After(t) && st.NextStart.IsZero():
			st.NextStart = iv.start
		}
	}
	return st
}

type interval struct{ start, end time.Time }

// intervals returns the occurrences of valid ranges from one day before t to
// eight days after it, sorted and with overlapping/adjacent ones merged.
// Starting a day back catches ranges that began yesterday and cross midnight;
// eight days forward always reaches the next weekly occurrence. Ranges that
// cover the whole horizon report CurrentEnd at its edge, fine for display.
func intervals(ranges []TimeRange, t time.Time, loc *time.Location) []interval {
	if loc == nil {
		loc = time.UTC
	}
	local := t.In(loc)
	base := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)

	var all []interval
	for _, r := range ranges {
		if r.Validate() != nil {
			continue
		}
		for offset := -1; offset <= 8; offset++ {
			day := base.AddDate(0, 0, offset)
			if r.onDay(day.Weekday()) {
				start, end := r.occurrence(day, loc)
				all = append(all, interval{start, end})
			}
		}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].start.Before(all[j].start) })

	var merged []interval
	for _, iv := range all {
		if n := len(merged); n > 0 && !iv.start.After(merged[n-1].end) {
			if iv.end.After(merged[n-1].end) {
				merged[n-1].end = iv.end
			}
			continue
		}
		merged = append(merged, iv)
	}
	return merged
}

func (r TimeRange) onDay(d time.Weekday) bool {
	for _, name := range r.Days {
		if dayIndex[name] == d {
			return true
		}
	}
	return false
}

// occurrence returns the range's start and end for the occurrence that starts
// on day (local midnight in loc). time.Date normalises wall-clock times that
// fall into a DST gap.
func (r TimeRange) occurrence(day time.Time, loc *time.Location) (time.Time, time.Time) {
	startMin, _ := parseClock(r.Start)
	endMin, _ := parseClock(r.End)
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, startMin, 0, 0, loc)
	endDay := day
	if endMin <= startMin {
		endDay = day.AddDate(0, 0, 1)
	}
	end := time.Date(endDay.Year(), endDay.Month(), endDay.Day(), 0, endMin, 0, 0, loc)
	return start, end
}

// parseClock parses "HH:MM" (00:00–23:59) into minutes after midnight.
func parseClock(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil || len(s) != 5 {
		return 0, fmt.Errorf("%q is not a time in HH:MM format", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}
