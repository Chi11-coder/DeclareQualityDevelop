package dqd

import "fmt"

// ResolvedRequirement は選択された品質要求とその適用理由を表す。
type ResolvedRequirement struct {
	Requirement Requirement `json:"requirement"`
	Reason      string      `json:"reason"`
}

// ResolveRequirements は常時適用・属性一致・明示指定から要求を選び、重複と個別合格条件の対応を確認する。
func ResolveRequirements(requirements []Requirement, ticket Ticket) ([]ResolvedRequirement, error) {
	known := make(map[string]bool)
	for _, req := range requirements {
		known[req.ID] = true
	}
	include := make(map[string]bool)
	for _, id := range ticket.Quality.Include {
		if !known[id] {
			return nil, fmt.Errorf("quality.includeの品質IDが未登録です: %s", id)
		}
		include[id] = true
	}
	attributes := make(map[string]bool)
	for _, attribute := range ticket.Attributes {
		attributes[attribute] = true
	}
	var result []ResolvedRequirement
	selected := make(map[string]bool)
	for _, req := range requirements {
		reason := ""
		switch req.Applicability.Mode {
		case "always":
			reason = "always"
		case "automatic":
			if len(req.Applicability.Attributes) != 1 {
				return nil, fmt.Errorf("%s: automaticには単一属性が必要です", req.ID)
			}
			if attributes[req.Applicability.Attributes[0]] {
				reason = "attribute: " + req.Applicability.Attributes[0]
			}
		case "explicit":
		default:
			return nil, fmt.Errorf("%s: 不明な適用方法です", req.ID)
		}
		if reason == "" && include[req.ID] {
			reason = "include"
		}
		if reason != "" {
			result = append(result, ResolvedRequirement{Requirement: req, Reason: reason})
			selected[req.ID] = true
		}
	}
	for id := range ticket.Criteria {
		if !known[id] {
			return nil, fmt.Errorf("個別合格条件の品質IDが未登録です: %s", id)
		}
		if !selected[id] {
			return nil, fmt.Errorf("個別合格条件の品質IDが適用対象にありません: %s", id)
		}
	}
	return result, nil
}
