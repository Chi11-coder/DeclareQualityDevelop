package dqd

import (
	"errors"
	"fmt"
	"path/filepath"
)

// Catalog は品質要求の宣言と、品質IDごとに抽出した本文を保持する。
type Catalog struct {
	Registry Registry
	Details  map[string]string
}

// ValidationSummary は検証した品質要求とチケットの件数を表す。
type ValidationSummary struct {
	Requirements int
	Tickets      int
}

// LoadCatalog は品質要求の宣言と全参照文書を読み込み、要求ごとの本文を検証して保持する。
func LoadCatalog(p *Project) (Catalog, error) {
	var catalog Catalog
	data, err := p.Read("requirements.yml")
	if err != nil {
		return catalog, err
	}
	catalog.Registry, err = ParseRegistry(data)
	if err != nil {
		return catalog, err
	}
	catalog.Details = make(map[string]string)
	cache := make(map[string]string)
	var issues []error
	for _, req := range catalog.Registry.Requirements {
		text, ok := cache[req.Document.Path]
		if !ok {
			data, readErr := p.Read(req.Document.Path)
			if readErr != nil {
				issues = append(issues, fmt.Errorf("%s: %w", req.ID, readErr))
				continue
			}
			text = string(data)
			cache[req.Document.Path] = text
		}
		detail, extractErr := ExtractRequirement(text, req.Document.Section)
		if extractErr == nil {
			_, extractErr = ShiftMarkdownHeadings(detail, 1)
		}
		if extractErr != nil {
			issues = append(issues, fmt.Errorf("%s (%s): %w", req.ID, req.Document.Path, extractErr))
			continue
		}
		catalog.Details[req.ID] = detail
	}
	return catalog, errors.Join(issues...)
}

// LoadTicket はチケットIDに対応するファイルを読み込み、ファイル名と宣言内容を検証する。
func LoadTicket(p *Project, id string) (Ticket, error) {
	if !ValidID(id) {
		return Ticket{}, fmt.Errorf("チケットIDが不正です: %q", id)
	}
	path := filepath.Join("tickets", id+".md")
	data, err := p.Read(path)
	if err != nil {
		return Ticket{}, err
	}
	result, err := ParseTicket(data, id)
	if err != nil {
		return result, fmt.Errorf("%s: %w", path, err)
	}
	return result, nil
}

// ResolveTicket はチケットの品質要求を解決し、必要な情報を欠かさずContextへ含められるか確認する。
func ResolveTicket(catalog Catalog, ticket Ticket) ([]ResolvedRequirement, string, error) {
	resolved, err := ResolveRequirements(catalog.Registry.Requirements, ticket)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", ticket.ID, err)
	}
	text, err := GenerateContext(ticket, resolved, catalog.Details)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", ticket.ID, err)
	}
	return resolved, text, nil
}

// ValidateProject は全品質要求と全チケットの整合性を検証し、件数と検出したエラーを返す。
func ValidateProject(p *Project) (ValidationSummary, error) {
	catalog, err := LoadCatalog(p)
	if err != nil {
		return ValidationSummary{}, err
	}
	if _, err := loadVerificationChecks(p, catalog.Registry); err != nil {
		return ValidationSummary{}, err
	}
	ids, err := p.TicketIDs()
	if err != nil {
		return ValidationSummary{}, err
	}
	var issues []error
	for _, id := range ids {
		ticket, err := LoadTicket(p, id)
		if err == nil {
			_, _, err = ResolveTicket(catalog, ticket)
		}
		if err != nil {
			issues = append(issues, err)
		}
	}
	return ValidationSummary{Requirements: len(catalog.Registry.Requirements), Tickets: len(ids)}, errors.Join(issues...)
}
