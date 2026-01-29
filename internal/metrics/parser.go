package metrics

import (
	claude "github.com/MateoSegura/claudesdk-go"
)

// StreamParser processes a stream of messages and extracts metrics.
type StreamParser struct {
	tracker    *Tracker
	initTools  []string
	costUSD    float64
	durationMS int64
	numTurns   int
	usage      *claude.Usage
	finalText  string
}

// ParserResult holds the assembled metrics from a parsed stream.
type ParserResult struct {
	InitTools  []string
	ToolCalls  []ToolCall
	Summary    map[string]int
	CostUSD    float64
	DurationMS int64
	NumTurns   int
	Usage      *claude.Usage
	FinalText  string
}

// NewStreamParser creates a parser with a fresh internal Tracker.
func NewStreamParser() *StreamParser {
	return &StreamParser{
		tracker: NewTracker(),
	}
}

// ProcessMessage processes a single stream message and updates internal state.
func (p *StreamParser) ProcessMessage(msg *claude.StreamMessage) {
	switch {
	case claude.IsInit(msg):
		p.initTools = claude.ExtractInitTools(msg)

	case claude.IsAssistant(msg):
		for _, block := range claude.GetAllToolCalls(msg) {
			p.tracker.Record(block.Name, block.Input)
		}

	case claude.IsResult(msg):
		p.costUSD = msg.CostUSD
		p.durationMS = msg.DurationMS
		p.numTurns = msg.NumTurns
		p.usage = msg.Usage
		p.finalText = msg.Result
	}
}

// Result returns the assembled parser result.
func (p *StreamParser) Result() ParserResult {
	return ParserResult{
		InitTools:  p.initTools,
		ToolCalls:  p.tracker.Calls(),
		Summary:    p.tracker.Summary(),
		CostUSD:    p.costUSD,
		DurationMS: p.durationMS,
		NumTurns:   p.numTurns,
		Usage:      p.usage,
		FinalText:  p.finalText,
	}
}
