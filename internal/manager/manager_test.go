package manager

import (
	"context"
	"testing"

	"github.com/maddydevel/HiveStack/internal/config"
	"github.com/maddydevel/HiveStack/internal/node"
)

type fakeController struct{ calls int }

func (f *fakeController) StartVM(context.Context, string) error   { f.calls++; return nil }
func (f *fakeController) StopVM(context.Context, string) error    { f.calls++; return nil }
func (f *fakeController) DestroyVM(context.Context, string) error { f.calls++; return nil }

func TestManager_vmController(t *testing.T) {
	agent, err := node.New(&config.NodeConfig{LibvirtURI: "test:///default"})
	if err != nil {
		t.Fatal(err)
	}
	remote := &fakeController{}

	m := &Manager{
		nodes:       map[string]*node.Agent{"local": agent, "both": agent},
		controllers: map[string]node.VMController{"remote": remote, "both": remote},
	}

	tests := []struct {
		name string
		host string
		want node.VMController
	}{
		{"unknown host", "missing", nil},
		{"local agent only", "local", agent},
		{"remote controller only", "remote", remote},
		{"remote takes precedence over local agent", "both", remote},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.vmController(tt.host); got != tt.want {
				t.Errorf("vmController(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}

	m.UnregisterNode("both")
	if got := m.vmController("both"); got != nil {
		t.Errorf("vmController after UnregisterNode = %v, want nil", got)
	}
}
