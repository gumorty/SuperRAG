// rag-queue-control pauses/resumes model-dependent enrichment during provider outages.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/Tencent/WeKnora/internal/common"
	"github.com/hibiken/asynq"
	"os"
	"strconv"
)

func main() {
	action := flag.String("action", "status", "status, pause, resume, snapshot or recover")
	manifest := flag.String("manifest", "", "reviewed recovery manifest; parent accounting must be restored first")
	wikiKB := flag.String("wake-wiki-kb", "", "run an existing orphan-claim recheck after its DB claims were released")
	flag.Parse()
	if *action != "status" && *action != "pause" && *action != "resume" && *action != "snapshot" && *action != "recover" {
		fmt.Fprintln(os.Stderr, "invalid action")
		os.Exit(1)
	}
	if os.Getenv("REDIS_ADDR") == "" {
		fmt.Fprintln(os.Stderr, "missing Redis address")
		os.Exit(1)
	}
	db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	i := asynq.NewInspector(asynq.RedisClientOpt{Addr: os.Getenv("REDIS_ADDR"), Username: os.Getenv("REDIS_USERNAME"), Password: os.Getenv("REDIS_PASSWORD"), DB: db, TLSConfig: common.RedisTLSConfig()})
	defer i.Close()
	if *wikiKB != "" {
		if err := wakeWiki(i, *wikiKB); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *action == "snapshot" || *action == "recover" {
		var err error
		if *action == "snapshot" {
			err = snapshotTasks(i)
		} else {
			err = recoverTasks(i, *manifest)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	for _, q := range []string{"graph", "wiki", "summary", "question", "multimodal"} {
		info, err := i.GetQueueInfo(q)
		if err != nil {
			continue
		} // queues not yet created have nothing to preserve
		if *action == "pause" && !info.Paused {
			err = i.PauseQueue(q)
		}
		if *action == "resume" && info.Paused {
			err = i.UnpauseQueue(q)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		info, err = i.GetQueueInfo(q)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		json.NewEncoder(os.Stdout).Encode(map[string]any{"queue": q, "paused": info.Paused, "active": info.Active, "pending": info.Pending, "retry": info.Retry, "archived": info.Archived, "completed": info.Completed})
	}
}
