package chatpipeline

import (
	"github.com/Tencent/WeKnora/internal/types"
	"testing"
)

func TestFollowUpRetrievalDecision(t *testing.T) {
	cases := []struct {
		name, output string
		want         bool
	}{
		{"new author facts", `{"intent":"follow_up","rewrite_query":"论文全部作者","needs_retrieval":true}`, true},
		{"format existing answer", `{"intent":"follow_up","rewrite_query":"把刚才答案改成表格","needs_retrieval":false}`, false},
		{"legacy classifier must not discard knowledge scope", `{"intent":"follow_up"}`, true},
		{"invalid boolean is not a false decision", `{"intent":"follow_up","needs_retrieval":"false"}`, true},
		{"null is unknown", `{"intent":"follow_up","needs_retrieval":null}`, true},
		{"truncated decision is conservative", `{"intent":"follow_up","needs_retrieval":fal`, true},
		{"explicit search cannot be disabled by contradictory flag", `{"intent":"kb_search","needs_retrieval":false}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cm := &types.ChatManage{PipelineRequest: types.PipelineRequest{KnowledgeBaseIDs: []string{"paper-kb"}}}
			(&PluginQueryUnderstand{}).parseOutput(cm, tc.output)
			if cm.NeedsRetrieval() != tc.want {
				t.Fatalf("retrieval=%v want=%v", cm.NeedsRetrieval(), tc.want)
			}
			clone := cm.Clone()
			if clone.NeedsRetrieval() != tc.want {
				t.Fatal("parallel search clone lost retrieval decision")
			}
			if clone.RetrievalRequired != nil {
				*clone.RetrievalRequired = !*clone.RetrievalRequired
				if cm.NeedsRetrieval() != tc.want {
					t.Fatal("clone mutated original decision")
				}
			}
		})
	}
}
