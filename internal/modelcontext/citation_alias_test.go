package modelcontext

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCitationAliasPreservesGroundingAndStreaming(t *testing.T) {
	r := newSourceRegistry()
	r.RegisterChunk(ChunkReference{ChunkID: "evidence-1", DocumentTitle: "Paper"})
	want := `claim <kb doc="Paper" chunk_id="evidence-1" />`
	require.Equal(t, want, r.ExpandText(`claim <cite id="c1"/>`))
	for split := 1; split < len(`claim <cite id="c1"/>`); split++ {
		s := newCitationStreamExpander(r)
		input := `claim <cite id="c1"/>`
		require.Equal(t, want, s.Feed(input[:split])+s.Feed(input[split:])+s.Flush())
	}
	require.Empty(t, r.ExpandText(`<cite id="c999"/>`))
	r.RegisterDocument("not-evidence")
	require.Empty(t, r.ExpandText(`<cite id="d1"/>`))
	rDisabled := newSourceRegistry(false)
	rDisabled.RegisterChunk(ChunkReference{ChunkID: "evidence-1"})
	require.Empty(t, rDisabled.ExpandText(`<cite id="c1"/>`))
	// A normal HTML cite element must remain untouched.
	require.Equal(t, `<cite>book title</cite>`, r.ExpandText(`<cite>book title</cite>`))
	s := newCitationStreamExpander(r)
	require.Equal(t, `<cite class="book">title</cite>`, s.Feed(`<ci`)+s.Feed(`te class="book">title</cite>`)+s.Flush())
}
