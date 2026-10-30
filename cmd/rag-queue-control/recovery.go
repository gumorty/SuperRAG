package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func wakeWiki(i *asynq.Inspector, kbID string) error {
	if _, err := uuid.Parse(kbID); err != nil {
		return fmt.Errorf("invalid KB ID")
	}
	q, err := i.GetQueueInfo("wiki")
	if err != nil {
		return err
	}
	if q.Active != 0 {
		return fmt.Errorf("wiki worker still active; cannot recover orphan claims")
	}
	id := "wiki-ingest-recheck-" + kbID
	task, err := i.GetTaskInfo("wiki", id)
	if err != nil {
		return err
	}
	if task.State != asynq.TaskStateScheduled {
		return fmt.Errorf("existing Wiki recheck must be scheduled")
	}
	if err := i.RunTask("wiki", id); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"wiki_recheck_awakened": id})
}

// Recovery is deliberately manifest-driven and requires paused queues. The
// operator must restore archived native tasks' parent slots before requeuing;
// this tool does not guess or silently change ingestion counters.
type recoveryTask struct {
	Queue         string `json:"queue"`
	ID            string `json:"id"`
	ExpectedError string `json:"expected_error,omitempty"`
}

func recoverTasks(i *asynq.Inspector, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var tasks []recoveryTask
	if err = json.Unmarshal(b, &tasks); err != nil {
		return err
	}
	if len(tasks) == 0 || len(tasks) > 2000 {
		return fmt.Errorf("bounded recovery manifest required")
	}
	seen := map[string]bool{}
	for _, task := range tasks {
		key := task.Queue + ":" + task.ID
		if (task.Queue != "graph" && task.Queue != "summary") || task.ID == "" || seen[key] {
			return fmt.Errorf("invalid recovery manifest")
		}
		seen[key] = true
		q, err := i.GetQueueInfo(task.Queue)
		if err != nil {
			return err
		}
		if !q.Paused || q.Active != 0 {
			return fmt.Errorf("queue %s must be paused and drained", task.Queue)
		}
		info, err := i.GetTaskInfo(task.Queue, task.ID)
		if err != nil {
			return err
		}
		if info.State != asynq.TaskStateArchived && info.State != asynq.TaskStatePending {
			return fmt.Errorf("task is not archived or already recovered")
		}
		if task.ExpectedError != "" {
			if info.LastErr != task.ExpectedError {
				return fmt.Errorf("task error changed since recovery review")
			}
		} else if !strings.Contains(info.LastErr, "balance is insufficient") {
			return fmt.Errorf("non-outage recovery requires the exact reviewed error")
		}
	}
	for _, task := range tasks {
		info, err := i.GetTaskInfo(task.Queue, task.ID)
		if err != nil {
			return err
		}
		if info.State == asynq.TaskStateArchived {
			if err = i.RunTask(task.Queue, task.ID); err != nil {
				return err
			}
		}
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"requeued_or_already_pending": len(tasks)})
}

func snapshotTasks(i *asynq.Inspector) error {
	enc := json.NewEncoder(os.Stdout)
	for _, queue := range []string{"graph", "summary"} {
		for _, state := range []string{"archived", "pending"} {
			for page := 1; ; page++ {
				var tasks []*asynq.TaskInfo
				var err error
				if state == "archived" {
					tasks, err = i.ListArchivedTasks(queue, asynq.PageSize(100), asynq.Page(page))
				} else {
					tasks, err = i.ListPendingTasks(queue, asynq.PageSize(100), asynq.Page(page))
				}
				if err != nil {
					return err
				}
				for _, task := range tasks {
					var raw map[string]any
					if err := json.Unmarshal(task.Payload, &raw); err != nil {
						return err
					}
					payload := map[string]any{}
					for _, key := range []string{"tenant_id", "knowledge_id", "knowledge_base_id", "chunk_id", "model_id", "attempt", "refresh"} {
						if v, ok := raw[key]; ok {
							payload[key] = v
						}
					}
					if err = enc.Encode(map[string]any{"queue": queue, "id": task.ID, "type": task.Type, "state": state, "retried": task.Retried, "max_retry": task.MaxRetry, "payload": payload, "last_error": task.LastErr}); err != nil {
						return err
					}
				}
				if len(tasks) < 100 {
					break
				}
			}
		}
	}
	return nil
}
