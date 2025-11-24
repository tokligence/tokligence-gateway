package scheduler

import (
	"testing"
	"time"
)

func TestTimeWindowWrapAroundRespectsPreviousDay(t *testing.T) {
	loc := time.FixedZone("TEST", 0)
	window := TimeWindow{
		StartHour:   18,
		StartMinute: 0,
		EndHour:     8,
		EndMinute:   0,
		DaysOfWeek:  []time.Weekday{time.Monday},
		Location:    loc,
	}

	mondayLate := time.Date(2024, time.January, 1, 23, 0, 0, 0, loc) // Monday
	if !window.IsActive(mondayLate) {
		t.Fatalf("expected window to be active Monday 23:00")
	}

	tuesdayEarly := time.Date(2024, time.January, 2, 7, 30, 0, 0, loc) // Tuesday morning should still count as Monday window
	if !window.IsActive(tuesdayEarly) {
		t.Fatalf("expected window to be active Tuesday 07:30 because it continues Monday window")
	}

	tuesdayLate := time.Date(2024, time.January, 2, 9, 0, 0, 0, loc)
	if window.IsActive(tuesdayLate) {
		t.Fatalf("expected window to be inactive Tuesday 09:00")
	}
}

func TestTimeWindowNonWrapDayOfWeek(t *testing.T) {
	loc := time.FixedZone("TEST", 0)
	window := TimeWindow{
		StartHour:   9,
		StartMinute: 0,
		EndHour:     10,
		EndMinute:   0,
		DaysOfWeek:  []time.Weekday{time.Wednesday},
		Location:    loc,
	}

	wed := time.Date(2024, time.January, 3, 9, 30, 0, 0, loc) // Wednesday
	if !window.IsActive(wed) {
		t.Fatalf("expected window to be active Wednesday 09:30")
	}

	thu := time.Date(2024, time.January, 4, 9, 30, 0, 0, loc) // Thursday
	if window.IsActive(thu) {
		t.Fatalf("expected window to be inactive Thursday 09:30")
	}
}
