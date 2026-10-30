// graph-backfill submits existing chunks to WeKnora's native graph worker.
// It does not reparse documents, regenerate chunks, or change vector indexes.
// Run inside the app container so Redis credentials stay in its environment.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Tencent/WeKnora/internal/common"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type manifest struct {
	TenantID uint64   `json:"tenant_id"`
	ModelID  string   `json:"model_id"`
	ChunkIDs []string `json:"chunk_ids"`
}

func run() error {
	input := flag.String("manifest", "", "JSON manifest of tenant, model and existing chunk IDs")
	status := flag.Bool("status", false, "inspect the existing tasks without enqueuing")
	flag.Parse()
	b, err := os.ReadFile(*input)
	if err != nil {
		return err
	}
	var m manifest
	if err = json.Unmarshal(b, &m); err != nil {
		return err
	}
	if m.TenantID == 0 || len(m.ChunkIDs) == 0 || len(m.ChunkIDs) > 1000 {
		return errors.New("invalid or unbounded manifest")
	}
	if _, err = uuid.Parse(m.ModelID); err != nil {
		return errors.New("invalid model ID")
	}
	seen := map[string]bool{}
	for _, id := range m.ChunkIDs {
		if _, err = uuid.Parse(id); err != nil || seen[id] {
			return errors.New("invalid or duplicate chunk ID")
		}
		seen[id] = true
	}
	if os.Getenv("NEO4J_ENABLE") != "true" {
		return errors.New("graph runtime must be enabled")
	}
	if os.Getenv("REDIS_ADDR") == "" {
		return errors.New("Redis must be configured")
	}
	db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	opt := asynq.RedisClientOpt{Addr: os.Getenv("REDIS_ADDR"), Username: os.Getenv("REDIS_USERNAME"), Password: os.Getenv("REDIS_PASSWORD"), DB: db, TLSConfig: common.RedisTLSConfig()}
	client := asynq.NewClient(opt)
	defer client.Close()
	inspector := asynq.NewInspector(opt)
	defer inspector.Close()
	enc := json.NewEncoder(os.Stdout)
	for _, id := range m.ChunkIDs {
		taskID := "graph-backfill-v1-" + id
		if !*status {
			// Omit KnowledgeID/Attempt intentionally: this is independent enrichment,
			// not a child of a running parse attempt. The worker resolves the document
			// from ChunkID and still applies its saved process overrides. This uses
			// the supported legacy payload path without draining ingestion counters.
			payload, err := json.Marshal(types.ExtractChunkPayload{TenantID: m.TenantID, ChunkID: id, ModelID: m.ModelID})
			if err != nil {
				return err
			}
			_, err = client.Enqueue(asynq.NewTask(types.TypeChunkExtract, payload), asynq.Queue(types.QueueGraph), asynq.TaskID(taskID), asynq.MaxRetry(3), asynq.Timeout(30*time.Minute), asynq.Retention(7*24*time.Hour))
			if err != nil && !errors.Is(err, asynq.ErrTaskIDConflict) {
				return err
			}
		}
		info, err := inspector.GetTaskInfo(types.QueueGraph, taskID)
		if err != nil {
			return err
		}
		if err = enc.Encode(map[string]any{"chunk_id": id, "task_id": taskID, "state": info.State.String(), "retried": info.Retried, "last_error": info.LastErr}); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
