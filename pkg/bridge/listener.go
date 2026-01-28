package bridge

import (
	"context"
	"log"
	"time"

	"github.com/zen-systems/flowgate/pkg/shm"
	"github.com/zen-systems/vtp-runtime/orchestrator"
)

type Listener struct {
	shm           *shm.SharedMemory
	orch          *orchestrator.Orchestrator
	entropyWindow []float32
	windowSize    int
}

const (
	MaxWindowSize = 10
	RiskThreshold = 4.5
)

func NewListener(shmPath string, orch *orchestrator.Orchestrator) (*Listener, error) {
	s, err := shm.Connect(shmPath)
	if err != nil {
		return nil, err
	}
	return &Listener{
		shm:        s,
		orch:       orch,
		windowSize: MaxWindowSize,
	}, nil
}

func (l *Listener) Close() {
	l.shm.Close()
}

func (l *Listener) ListenForSignals() {
	log.Println("Starting Bridge Listener...")
	for {
		resp, err := l.shm.PollResponse()
		if err != nil {
			log.Printf("Error polling response: %v", err)
			time.Sleep(1 * time.Second) // Backoff on error
			continue
		}

		if resp != nil {
			// Response received
			// Log regular responses
			if resp.Status != shm.RSP_CODE_STUB {
				log.Printf("Received response: Cmd=%d Status=%x Result=%d", resp.OrigCmd, resp.Status, resp.Result)
			}
			// Special handling for Code Stub is already checked inside PollResponse for simple trigger,
			// but here we can add higher-level logic if needed.
			if resp.Status == shm.RSP_CODE_STUB {
				// 1. Update Sliding Window
				l.updateEntropyWindow(resp.EntropyAvg)

				// 2. Compute L1 Risk
				riskScore := l.computeL1Risk()

				log.Printf("Detected Code Stub (Ent=%.2f). L1 Risk Score: %.2f", resp.EntropyAvg, riskScore)

				if riskScore > RiskThreshold {
					log.Printf("CRITICAL: Risk Score %.2f > Threshold. Invoking VTP Repair.", riskScore)

					hints := []string{"High Entropy Hallucination Detected"}
					if resp.Result == 202 {
						hints = []string{"Low Entropy / Lazy Stub Detected"}
					}

					// TODO: Get active Evaluation ID from Orchestrator state or SHM payload
					activeEvalID := "eval-latest"

					// Trigger Graft
					ctx := context.Background()
					// We pass the RiskScore as entropy for now, or update GraftByHint signature to take RiskScore.
					// Existing GraftByHint takes (ctx, evalID, integrityCode, entropy, hints)
					_, _, err := l.orch.GraftByHint(ctx, activeEvalID, resp.Result, riskScore, hints)
					if err != nil {
						log.Printf("Failed to trigger graft: %v", err)
					} else {
						log.Printf("Speculative Repair Triggered Successfully.")
					}
				}
			}
		}

		// PollResponse already sleeps if empty, so we don't need a sleep here
	}
}

func (l *Listener) updateEntropyWindow(val float32) {
	l.entropyWindow = append(l.entropyWindow, val)
	if len(l.entropyWindow) > l.windowSize {
		l.entropyWindow = l.entropyWindow[1:]
	}
}

func (l *Listener) computeL1Risk() float32 {
	if len(l.entropyWindow) == 0 {
		return 0.0
	}
	var sum float32
	for _, v := range l.entropyWindow {
		sum += v
	}
	mean := sum / float32(len(l.entropyWindow))

	// Simple risk: if mean is high, risk is high.
	// We could add "sustained" factor: if all recent values > X then multiplier.
	return mean
}
