package ui

import (
	"perch/internal/query"
	"strings"
	"testing"
)

func TestModuleCopyReadAndScopeGuards(t *testing.T) {
	m := demoModel(80, 24)
	defer m.Close()
	c := m.showSoftware("module", "cuda/12.8.1")
	if m.action("y") != nil {
		t.Fatal("copied while loading")
	}
	m.Update(c())
	if m.moduleInfo == nil || m.moduleInfo.LoadCommand != "module load cuda/12.8.1" {
		t.Fatal("missing typed detail")
	}
	for _, key := range []string{"copy-failed"} {
		if m.action(key) != nil || !strings.Contains(m.notice, "Choose a job") {
			t.Fatal("array command copied module", m.notice)
		}
	}
	good := *m.moduleInfo
	for _, bad := range []query.ModuleInfo{
		{Name: "other/1", LoadCommand: "module load other/1"},
		{Name: good.Name, LoadCommand: "module load cuda/12.8.1; touch /tmp/no"},
		{Name: "cuda/12.8.1;bad", LoadCommand: "module load cuda/12.8.1;bad"},
	} {
		m.moduleInfo = &bad
		if m.action("y") != nil {
			t.Fatal("copied unexpected detail", bad)
		}
	}
	m.moduleInfo = &good
	m.detailError = "read failed"
	if m.action("y") != nil {
		t.Fatal("copied failed reread")
	}
	m.detailError = ""
	m.invalidateDetail()
	if m.action("y") != nil || m.moduleInfo != nil {
		t.Fatal("stale detail survived invalidation")
	}
}
