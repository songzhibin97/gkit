package distributed

import "testing"

// Regression for #166 / 03-06: each worker must inherit the server's explicit
// signal-ownership setting while retaining its own creation arguments.
func TestNewWorkerCopiesNoUnixSignals(t *testing.T) {
	for name, disabled := range map[string]bool{"enabled": false, "disabled": true} {
		t.Run(name, func(t *testing.T) {
			config := &Config{}
			SetNoUnixSignals(disabled)(config)
			server := &Server{config: config}
			worker := server.NewWorker("consumer", 3, "queue")
			if worker.NoUnixSignals != disabled {
				t.Errorf("NoUnixSignals = %t, want %t", worker.NoUnixSignals, disabled)
			}
			if worker.bindService != server || worker.ConsumerTag != "consumer" || worker.Concurrency != 3 || worker.Queue != "queue" {
				t.Fatalf("worker creation arguments changed: %#v", worker)
			}
			SetNoUnixSignals(!disabled)(server.GetConfig())
			if worker.NoUnixSignals != disabled {
				t.Error("existing worker setting changed after server reconfiguration")
			}
			if got := server.NewWorker("next", 1, "other").NoUnixSignals; got != !disabled {
				t.Errorf("new worker setting = %t, want %t", got, !disabled)
			}
		})
	}
	if worker := (&Server{}).NewWorker("default", 1, "queue"); worker.NoUnixSignals {
		t.Fatal("zero-value server enabled NoUnixSignals")
	}
}
