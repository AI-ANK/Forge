package agent

import (
	"encoding/json"
	"fmt"

	"github.com/AI-ANK/Forge/internal/llm"
	"github.com/AI-ANK/Forge/internal/session"
	"github.com/AI-ANK/Forge/internal/tools"
)

// ForkPrep reconstructs chat history up to the given turn from a parent
// session's events, copies those events verbatim into the child recorder,
// and returns the message slice the live loop should resume from plus
// the next turn index.
//
// `atTurn` is 1-indexed (the turn at which the user wants to diverge).
// The reconstructed history includes turn atTurn's recorded assistant
// response and tool result (if any), so the fork diverges *after* step
// atTurn. Setting atTurn = 0 means "fork from the very beginning" (only
// the initial user goal is kept).
func ForkPrep(
	parentSess *session.Session,
	parentEvents []session.Event,
	child *session.Recorder,
	reg *tools.Registry,
	cwd, newGuidance string,
	atTurn int,
) (messages []llm.Message, nextTurn int, err error) {
	system := SystemPrompt(reg, parentSess.Goal, cwd)
	messages = []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: parentSess.Goal},
	}

	// The child session records everything fresh so it's self-contained.
	if err := child.Record(session.KindUserMsg, map[string]string{"goal": parentSess.Goal}); err != nil {
		return nil, 0, err
	}

	turn := 0
	var pendingAssistant string
	var pendingToolName string
	for _, ev := range parentEvents {
		if turn > atTurn {
			break
		}
		switch ev.Kind {
		case session.KindUserMsg:
			// already seeded from parent goal
		case session.KindLLMRequest:
			if turn < atTurn {
				if err := child.RecordRaw(session.KindLLMRequest, ev.Payload); err != nil {
					return nil, 0, err
				}
			}
		case session.KindLLMResponse:
			turn++
			if turn > atTurn {
				break
			}
			var resp llm.Response
			if err := json.Unmarshal(ev.Payload, &resp); err != nil {
				return nil, 0, fmt.Errorf("decode llm_resp at turn %d: %w", turn, err)
			}
			pendingAssistant = resp.Content
			if err := child.RecordRaw(session.KindLLMResponse, ev.Payload); err != nil {
				return nil, 0, err
			}
			// Parse to know whether we expect a tool_result next.
			var act Action
			_ = json.Unmarshal([]byte(pendingAssistant), &act)
			pendingToolName = act.Tool
			messages = append(messages, llm.Message{Role: "assistant", Content: pendingAssistant})
			if act.Final != "" {
				// Parent run finished at or before atTurn. Fork has nothing to continue.
				return messages, turn + 1, fmt.Errorf("cannot fork: parent session ended at turn %d", turn)
			}
		case session.KindToolCall:
			if turn <= atTurn {
				if err := child.RecordRaw(session.KindToolCall, ev.Payload); err != nil {
					return nil, 0, err
				}
			}
		case session.KindToolResult:
			if turn > atTurn {
				break
			}
			var tr struct {
				Tool   string `json:"tool"`
				Result string `json:"result"`
			}
			if err := json.Unmarshal(ev.Payload, &tr); err != nil {
				return nil, 0, fmt.Errorf("decode tool_result at turn %d: %w", turn, err)
			}
			if err := child.RecordRaw(session.KindToolResult, ev.Payload); err != nil {
				return nil, 0, err
			}
			name := tr.Tool
			if name == "" {
				name = pendingToolName
			}
			messages = append(messages, llm.Message{Role: "tool", Name: name, Content: tr.Result})
		case session.KindAgentFinal:
			return messages, turn + 1, fmt.Errorf("cannot fork: parent session ended at turn %d", turn)
		}
	}
	if turn < atTurn {
		return nil, 0, fmt.Errorf("parent session has only %d turns; cannot fork at turn %d", turn, atTurn)
	}

	// Append the user's corrective guidance so the next live turn sees it.
	guidance := fmt.Sprintf("Updated guidance from the user: %s", newGuidance)
	messages = append(messages, llm.Message{Role: "user", Content: guidance})
	if err := child.Record(session.KindUserMsg, map[string]string{"goal": guidance}); err != nil {
		return nil, 0, err
	}

	return messages, atTurn + 1, nil
}
