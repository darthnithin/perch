package hummingbird

import (
	"context"
	"perch/internal/transport"
	"strings"
	"testing"
)

// Synthetic fixture in the explicit sprio format; not an unmodified live capture.
const priorityFixture = "802978|researcher|128x24|10010046|0|1|0|0|45|10000|10000000|0|\n"

func TestPriorityPreservesTotalWhenDisplayedFactorsRound(t *testing.T) {
	// The production probe reported 10010022 while displayed components summed
	// to 10010023. IDs/user are synthetic; retain that observed numeric boundary.
	rows, err := parsePriority("42|researcher|128x24|10010022|0|0|0|0|23|10000|10000000|0|\n", "researcher")
	if err != nil || len(rows) != 1 || rows[0].Score != 10010022 || rows[0].Factors[4].Value != 23 {
		t.Fatal(rows, err)
	}
}

func TestPriorityFactorsStayWeightedAndPartitionScoped(t *testing.T) {
	text := priorityFixture + strings.Replace(strings.TrimSpace(priorityFixture), "128x24", "256x44", 1) + "cpu=1.5,gres/gpu=20\n"
	rows, err := parsePriority(text, "researcher")
	if err != nil || len(rows) != 2 {
		t.Fatal(rows, err)
	}
	if rows[0].Score != 10010046 || rows[0].Factors[3].Name != "Fairshare" || rows[0].Factors[3].Value != 0 {
		t.Fatal(rows[0])
	}
	if len(rows[1].Factors) != 9 || rows[1].Factors[7].Value != 1.5 {
		t.Fatal(rows[1])
	}
	empty, err := parsePriority("\n", "researcher")
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatal(empty, err)
	}
}
func TestPriorityRejectsMalformedOrForeignRows(t *testing.T) {
	for _, tc := range []struct{ old, new string }{{"researcher", "other"}, {"10010046", "NaN"}, {"|45|", "|-1|"}, {"|10000|", "|Inf|"}, {"802978", "802978_1"}, {"128x24", "bad\x1b[2J"}, {"|0|\n", "|bad|\n"}} {
		if rows, err := parsePriority(strings.Replace(priorityFixture, tc.old, tc.new, 1), "researcher"); err == nil || rows != nil {
			t.Fatal(tc, rows, err)
		}
	}
	for _, text := range []string{"Welcome to the server", priorityFixture + priorityFixture, strings.TrimSpace(priorityFixture) + "cpu=1,cpu=2", strings.TrimSpace(priorityFixture) + "gpu=NaN", strings.Repeat("x", (2<<20)+1)} {
		if _, err := parsePriority(text, "researcher"); err == nil {
			t.Fatal("malformed response accepted")
		}
	}
}
func TestPriorityUsesExplicitScopedCommand(t *testing.T) {
	r := &fakeRunner{results: []transport.Result{{Stdout: priorityFixture}}}
	c := Client{Runner: r, User: "researcher"}
	got, err := c.Priority(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
	command := strings.Join(r.commands[0], " ")
	for _, want := range []string{"--local", "--user=researcher", "--format=" + priorityFormat, "-u SLURM_CLUSTERS", "-u SPRIO_SIBLING"} {
		if !strings.Contains(command, want) {
			t.Fatal(command)
		}
	}
	c.User = ""
	if _, err = c.Priority(context.Background()); err == nil || len(r.commands) != 1 {
		t.Fatal("queried without identity")
	}
	r = &fakeRunner{results: []transport.Result{{ExitCode: 1, Stderr: "unsupported plugin"}}}
	if _, err = (&Client{Runner: r, User: "researcher"}).Priority(context.Background()); err == nil {
		t.Fatal("failure became empty success")
	}
}

func TestPriorityPolicyUsesConfigurationAndKeepsPartialSuccess(t *testing.T) {
	for _, tc := range []struct {
		config string
		code   int
		known  bool
		weight uint64
	}{
		{"PriorityType = priority/multifactor\nPriorityWeightFairShare = 0\n", 0, true, 0},
		{"PriorityType = priority/multifactor\nPriorityWeightFairShare = 10000\n", 0, true, 10000},
		{"PriorityType = priority/basic\nPriorityWeightFairShare = 0\n", 0, false, 0},
		{"PriorityType = priority/multifactor\n", 0, false, 0},
		{"PriorityType = priority/multifactor\nPriorityWeightFairShare = -1\n", 0, false, 0},
		{"PriorityType = priority/multifactor\nPriorityWeightFairShare = 4294967296\n", 0, false, 0},
		{"PriorityType = priority/multifactor\nPriorityWeightFairShare = 0\n", 1, false, 0},
	} {
		r := &fakeRunner{results: []transport.Result{{Stdout: priorityFixture}, {Stdout: tc.config, ExitCode: tc.code}}}
		rows, policy, err := (&Client{Runner: r, User: "researcher"}).PriorityWithPolicy(context.Background())
		if err != nil || len(rows) != 1 || (policy.FairshareWeight != nil) != tc.known {
			t.Fatal(tc, rows, policy, err)
		}
		if tc.known && *policy.FairshareWeight != tc.weight {
			t.Fatal(policy)
		}
		if tc.known && tc.weight == 0 && !strings.Contains(policy.Note, "does not contribute") {
			t.Fatal(policy)
		}
		if !tc.known && !strings.Contains(policy.Note, "unavailable") {
			t.Fatal(policy)
		}
		if len(r.commands) != 2 || !strings.Contains(strings.Join(r.commands[0], " "), "--user=researcher") || strings.Join(r.commands[1], " ") != "scontrol show config" {
			t.Fatal(r.commands)
		}
	}
	r := &fakeRunner{results: []transport.Result{{ExitCode: 1, Stderr: "priority failed"}, {}}}
	if _, _, err := (&Client{Runner: r, User: "researcher"}).PriorityWithPolicy(context.Background()); err == nil {
		t.Fatal("required scores failed silently")
	}
	r.commands = nil
	if _, _, err := (&Client{Runner: r}).PriorityWithPolicy(context.Background()); err == nil || len(r.commands) != 0 {
		t.Fatal("queried without user")
	}
}
