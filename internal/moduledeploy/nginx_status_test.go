package moduledeploy

import (
	"path/filepath"
	"testing"

	dockerx "github.com/wpg/wpgctl/internal/docker"
)

func TestInspectNginxRuntime_missingDir(t *testing.T) {
	st := InspectNginxRuntime(filepath.Join(t.TempDir(), "missing"))
	if st.Deployed {
		t.Fatalf("missing dir must not be deployed: %+v", st)
	}
}

func TestJudgeNginxRuntime_allRunning(t *testing.T) {
	st := JudgeNginxRuntime(NginxRuntime{ModuleDir: "/workspace/nginx"}, []dockerx.ComposeService{
		{Name: "nginx", Service: "nginx", State: "running", Status: "Up 2 hours"},
	})
	if !st.Deployed || st.Running != 1 || st.Total != 1 {
		t.Fatalf("want deployed 1/1, got %+v", st)
	}
	if st.Summary == "" || st.Reason != "" {
		t.Fatalf("summary/reason: %+v", st)
	}
}

func TestJudgeNginxRuntime_empty(t *testing.T) {
	st := JudgeNginxRuntime(NginxRuntime{}, nil)
	if st.Deployed || st.Reason == "" {
		t.Fatalf("empty should not be deployed: %+v", st)
	}
}

func TestJudgeNginxRuntime_unhealthy(t *testing.T) {
	st := JudgeNginxRuntime(NginxRuntime{}, []dockerx.ComposeService{
		{Service: "nginx", State: "running", Health: "unhealthy"},
	})
	if st.Deployed {
		t.Fatal("unhealthy must not count as deployed")
	}
}

func TestJudgeNginxRuntime_partial(t *testing.T) {
	st := JudgeNginxRuntime(NginxRuntime{}, []dockerx.ComposeService{
		{Service: "nginx", State: "running"},
		{Service: "sidecar", State: "exited"},
	})
	if st.Deployed || st.Running != 1 || st.Total != 2 {
		t.Fatalf("partial: %+v", st)
	}
}
