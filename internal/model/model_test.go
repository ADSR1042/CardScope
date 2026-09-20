package model

import "testing"

func TestIdleRequiresAllSignals(t *testing.T) {
	g := GPU{
		Status:        "ok",
		ProcessStatus: "ok",
		Util:          Number(0),
		MemoryTotal:   Number(2 << 30),
		MemoryUsed:    Number(16 << 20),
	}
	if !Idle(g) {
		t.Fatal("idle")
	}
	g.MemoryUsed = Number(1668 << 20)
	if Idle(g) {
		t.Fatal("occupied memory considered idle")
	}
	g.MemoryUsed = Number(0)
	g.ProcessStatus = "wsl_limited"
	if Idle(g) {
		t.Fatal("WSL unknown considered idle")
	}
	g.ProcessStatus = "ok"
	g.Util = nil
	if Idle(g) {
		t.Fatal("null considered idle")
	}
}
