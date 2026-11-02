package main

import (
	"slices"
	"testing"

	"github.com/mr-tortilla/brod/internal/node"
)

func TestCarryPinned(t *testing.T) {
	oldNodes := []node.Node{
		{ID: "old-a", Name: "A", Server: "1.1.1.1", Port: 443},
		{ID: "b", Name: "B", Server: "2.2.2.2", Port: 443},
		{ID: "gone", Name: "C", Server: "3.3.3.3", Port: 443},
	}
	newNodes := []node.Node{
		{ID: "b", Name: "B", Server: "2.2.2.2", Port: 443},
		{ID: "new-a", Name: "A", Server: "1.1.1.1", Port: 443},
	}
	got := carryPinned([]string{"gone", "old-a", "b"}, oldNodes, newNodes)
	if want := []string{"new-a", "b"}; !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseArgs(t *testing.T) {
	a := parseArgs([]string{"--wait-pid", "1234", "--connect", "p1:n1:tun", "--unknown"})
	if a.waitPID != 1234 || a.connect == nil || *a.connect != (StartupAction{ProfileID: "p1", NodeID: "n1", Mode: "tun"}) {
		t.Fatalf("%+v %+v", a, a.connect)
	}
	if b := parseArgs(nil); b.waitPID != 0 || b.connect != nil {
		t.Fatalf("пустые аргументы: %+v", b)
	}
}
