package main

import (
	"testing"
	"time"
)

func TestComputeNextRunInterval(t *testing.T) {
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	task := ScheduledTask{ScheduleType: "interval", ScheduleValue: "2h"}
	next, ok := computeNextRun(task, now)
	if !ok {
		t.Fatal("expected ok")
	}
	want := now.Add(2 * time.Hour).UnixMilli()
	if next != want {
		t.Fatalf("next = %d, want %d", next, want)
	}
}

func TestComputeNextRunDailySameDay(t *testing.T) {
	now := time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC)
	task := ScheduledTask{ScheduleType: "daily", ScheduleValue: "09:00"}
	next, ok := computeNextRun(task, now)
	if !ok {
		t.Fatal("expected ok")
	}
	want := time.Date(2026, 6, 7, 9, 0, 0, 0, time.UTC).UnixMilli()
	if next != want {
		t.Fatalf("next = %d, want %d", next, want)
	}
}

func TestComputeNextRunDailyTomorrow(t *testing.T) {
	now := time.Date(2026, 6, 7, 10, 0, 0, 0, time.UTC)
	task := ScheduledTask{ScheduleType: "daily", ScheduleValue: "09:00"}
	next, ok := computeNextRun(task, now)
	if !ok {
		t.Fatal("expected ok")
	}
	want := time.Date(2026, 6, 8, 9, 0, 0, 0, time.UTC).UnixMilli()
	if next != want {
		t.Fatalf("next = %d, want %d", next, want)
	}
}

func TestAdvanceOneTimeDisablesTask(t *testing.T) {
	task := ScheduledTask{
		ID:           "task-1",
		ScheduleType: "one-time",
		Enabled:      true,
		NextRun:      time.Now().UnixMilli(),
	}
	updated := advanceScheduledTaskAfterRun(task, time.Now())
	if updated.Enabled {
		t.Fatal("one-time task should disable after run")
	}
	if updated.NextRun != 0 {
		t.Fatalf("nextRun = %d, want 0", updated.NextRun)
	}
}

func TestParseScheduleInterval(t *testing.T) {
	d, err := parseScheduleInterval("30m")
	if err != nil {
		t.Fatal(err)
	}
	if d != 30*time.Minute {
		t.Fatalf("duration = %v", d)
	}
}
