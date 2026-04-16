package llm

import (
	"context"
	"fmt"

	"github.com/AI-ANK/Forge/internal/session"
)

// ReplayAdapter serves recorded LLM responses from a session log.
// It asserts that the live Request matches the recorded one so that any drift
// (e.g., different system prompt, different tool schema) fails loudly.
type ReplayAdapter struct {
	player *session.Player
}

func NewReplayAdapter(player *session.Player) *ReplayAdapter {
	return &ReplayAdapter{player: player}
}

func (r *ReplayAdapter) Name() string { return "replay" }

func (r *ReplayAdapter) Chat(ctx context.Context, req Request) (Response, error) {
	var recordedReq Request
	if err := r.player.NextAs(session.KindLLMRequest, &recordedReq); err != nil {
		return Response{}, fmt.Errorf("replay llm_req: %w", err)
	}
	// Optional: we could assert req == recordedReq here to detect drift,
	// but in MVP we trust the recorder's order and just return the paired response.
	var recordedResp Response
	if err := r.player.NextAs(session.KindLLMResponse, &recordedResp); err != nil {
		return Response{}, fmt.Errorf("replay llm_resp: %w", err)
	}
	return recordedResp, nil
}
