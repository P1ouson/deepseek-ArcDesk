package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	scheduleTickInterval = 30 * time.Second
	scheduleEventName    = "schedule:task"
)

// taskScheduler runs enabled scheduled tasks in the background.
type taskScheduler struct {
	app  *App
	stop chan struct{}
	wg   sync.WaitGroup
}

func (a *App) startTaskScheduler() {
	if a == nil {
		return
	}
	a.sched = &taskScheduler{app: a, stop: make(chan struct{})}
	a.sched.wg.Add(1)
	go a.sched.loop()
}

func (a *App) stopTaskScheduler() {
	if a == nil || a.sched == nil {
		return
	}
	close(a.sched.stop)
	a.sched.wg.Wait()
	a.sched = nil
}

func (s *taskScheduler) loop() {
	defer s.wg.Done()
	ticker := time.NewTicker(scheduleTickInterval)
	defer ticker.Stop()

	s.tick()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

func (s *taskScheduler) tick() {
	if s == nil || s.app == nil {
		return
	}
	now := time.Now()
	path := ARCDESKDesktopDataPath("scheduled-tasks.json")
	items, err := loadScheduledTasks(path)
	if err != nil || len(items) == 0 {
		return
	}
	changed := false
	for i := range items {
		task := items[i]
		if !task.Enabled || task.ScheduleType == "manual" {
			continue
		}
		if task.NextRun <= 0 {
			if next, ok := computeNextRun(task, now); ok {
				items[i].NextRun = next
				changed = true
			}
			continue
		}
		if task.NextRun > now.UnixMilli() {
			continue
		}
		if err := s.app.runScheduledTask(task, "auto"); err != nil {
			s.app.emitScheduleFailure(task, err)
			continue
		}
		items[i] = advanceScheduledTaskAfterRun(task, now)
		changed = true
	}
	if changed {
		_ = saveJSON(path, items)
	}
}

func (a *App) emitScheduleFailure(task ScheduledTask, err error) {
	if a == nil || a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, scheduleEventName, map[string]any{
		"id":     task.ID,
		"name":   task.Name,
		"source": "auto",
		"error":  err.Error(),
	})
}

func (a *App) runScheduledTaskByID(id, source string) error {
	key := strings.TrimSpace(id)
	if key == "" {
		return fmt.Errorf("task id is required")
	}
	items, err := loadScheduledTasks(ARCDESKDesktopDataPath("scheduled-tasks.json"))
	if err != nil {
		return err
	}
	var task *ScheduledTask
	for i := range items {
		if items[i].ID == key {
			task = &items[i]
			break
		}
	}
	if task == nil {
		return fmt.Errorf("scheduled task not found: %s", key)
	}
	if err := a.runScheduledTask(*task, source); err != nil {
		return err
	}
	for i := range items {
		if items[i].ID != key {
			continue
		}
		items[i] = advanceScheduledTaskAfterRun(items[i], time.Now())
		break
	}
	return saveJSON(ARCDESKDesktopDataPath("scheduled-tasks.json"), items)
}

func (a *App) runScheduledTask(task ScheduledTask, source string) error {
	prompt := strings.TrimSpace(task.Prompt)
	if prompt == "" {
		return fmt.Errorf("task prompt is empty")
	}
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, scheduleEventName, map[string]any{
			"id":     task.ID,
			"name":   task.Name,
			"source": source,
			"prompt": prompt,
		})
	}

	root := strings.TrimSpace(task.WorkspaceRoot)
	var tabMeta TabMeta
	var err error
	title := strings.TrimSpace(task.Name)
	if title == "" {
		title = "Scheduled task"
	}
	if root == "" || root == "." {
		topic, topicErr := a.CreateTopic("global", "", title)
		if topicErr != nil {
			return topicErr
		}
		tabMeta, err = a.OpenGlobalTab(topic.ID)
	} else {
		topic, topicErr := a.CreateTopic("project", root, title)
		if topicErr != nil {
			return topicErr
		}
		tabMeta, err = a.OpenProjectTab(root, topic.ID)
	}
	if err != nil {
		return err
	}
	if model := strings.TrimSpace(task.Model); model != "" {
		_ = a.SetModelForTab(tabMeta.ID, model)
	}
	a.SubmitToTab(tabMeta.ID, prompt)
	return nil
}

func normalizeScheduledTask(task ScheduledTask, now time.Time) ScheduledTask {
	if task.ScheduleType == "manual" {
		return task
	}
	if next, ok := computeNextRun(task, now); ok {
		task.NextRun = next
	}
	return task
}

func advanceScheduledTaskAfterRun(task ScheduledTask, now time.Time) ScheduledTask {
	switch task.ScheduleType {
	case "one-time":
		task.Enabled = false
		task.NextRun = 0
	case "manual":
		// unchanged
	default:
		if next, ok := computeNextRun(task, now); ok {
			task.NextRun = next
		}
	}
	return task
}

func computeNextRun(task ScheduledTask, from time.Time) (int64, bool) {
	switch task.ScheduleType {
	case "manual":
		return 0, false
	case "interval":
		d, err := parseScheduleInterval(task.ScheduleValue)
		if err != nil {
			d = time.Hour
		}
		return from.Add(d).UnixMilli(), true
	case "daily":
		hour, minute, err := parseDailyClock(task.ScheduleValue)
		if err != nil {
			hour, minute = 9, 0
		}
		next := nextDailyOccurrence(from, hour, minute)
		return next.UnixMilli(), true
	case "one-time":
		at, err := parseOneTime(task.ScheduleValue, from)
		if err != nil {
			return 0, false
		}
		return at.UnixMilli(), true
	default:
		return from.Add(time.Hour).UnixMilli(), true
	}
}

func parseScheduleInterval(raw string) (time.Duration, error) {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return 0, fmt.Errorf("empty interval")
	}
	multiplier := time.Minute
	if strings.HasSuffix(value, "ms") {
		multiplier = time.Millisecond
		value = strings.TrimSuffix(value, "ms")
	} else if strings.HasSuffix(value, "s") {
		multiplier = time.Second
		value = strings.TrimSuffix(value, "s")
	} else if strings.HasSuffix(value, "m") {
		multiplier = time.Minute
		value = strings.TrimSuffix(value, "m")
	} else if strings.HasSuffix(value, "h") {
		multiplier = time.Hour
		value = strings.TrimSuffix(value, "h")
	} else if strings.HasSuffix(value, "d") {
		multiplier = 24 * time.Hour
		value = strings.TrimSuffix(value, "d")
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid interval %q", raw)
	}
	return time.Duration(n) * multiplier, nil
}

func parseDailyClock(raw string) (hour, minute int, err error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, 0, fmt.Errorf("empty daily time")
	}
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid daily time %q", raw)
	}
	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour in %q", raw)
	}
	minute, err = strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute in %q", raw)
	}
	return hour, minute, nil
}

func nextDailyOccurrence(from time.Time, hour, minute int) time.Time {
	loc := from.Location()
	candidate := time.Date(from.Year(), from.Month(), from.Day(), hour, minute, 0, 0, loc)
	if !candidate.After(from) {
		candidate = candidate.Add(24 * time.Hour)
	}
	return candidate
}

func parseOneTime(raw string, fallback time.Time) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty one-time value")
	}
	if ms, err := strconv.ParseInt(value, 10, 64); err == nil {
		if ms > 1_000_000_000_000 {
			return time.UnixMilli(ms), nil
		}
		return time.Unix(ms, 0), nil
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04",
		"2006-01-02T15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, value, fallback.Location()); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid one-time value %q", raw)
}
