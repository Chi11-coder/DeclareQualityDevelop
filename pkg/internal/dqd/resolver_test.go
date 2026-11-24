package dqd

import (
	"reflect"
	"testing"
)

// TestAlwaysAutomaticAndExplicitAreDeduplicatedInRegistryOrder は適用条件と明示指定を重複なく宣言順に解決し、未知・非適用IDの条件を拒否することを検証する。
func TestAlwaysAutomaticAndExplicitAreDeduplicatedInRegistryOrder(t *testing.T) {
	requirements := []Requirement{
		{ID: "CM-001", Domain: "common", Applicability: Applicability{Mode: "always"}},
		{ID: "UI-001", Domain: "frontend", Applicability: Applicability{Mode: "automatic", Attributes: []string{"screen"}}},
		{ID: "BE-001", Domain: "backend", Applicability: Applicability{Mode: "automatic", Attributes: []string{"data_update"}}},
		{ID: "PF-001", Domain: "backend", Applicability: Applicability{Mode: "explicit"}},
	}
	task := Ticket{TicketMetadata: TicketMetadata{Attributes: []string{"screen"}, Domains: []string{"frontend"}}, Criteria: map[string]string{"PF-001": "条件"}}
	task.Quality.Include = []string{"CM-001", "UI-001", "PF-001", "PF-001"}
	resolved, err := ResolveRequirements(requirements, task)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, item := range resolved {
		ids = append(ids, item.Requirement.ID)
	}
	if !reflect.DeepEqual(ids, []string{"CM-001", "UI-001", "PF-001"}) {
		t.Fatalf("適用結果が不正です: %v", ids)
	}
	if resolved[1].Reason != "attribute: screen" || resolved[2].Reason != "include" {
		t.Fatalf("適用理由が不正です: %+v", resolved)
	}
	// 対象領域は分類情報のため、明示指定したbackendの要求も適用対象になる。
	task.Quality.Include = []string{"UNKNOWN"}
	if _, err := ResolveRequirements(requirements, task); err == nil {
		t.Fatal("未知の品質IDはエラーが必要です")
	}
	task.Quality.Include = nil
	task.Criteria = map[string]string{"BE-001": "条件"}
	if _, err := ResolveRequirements(requirements, task); err == nil {
		t.Fatal("未適用の個別合格条件はエラーが必要です")
	}
}
