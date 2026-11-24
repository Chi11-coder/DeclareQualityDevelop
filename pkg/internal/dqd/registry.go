package dqd

import (
	"errors"
	"fmt"
	"path/filepath"
)

// Registry はrequirements.ymlに登録された品質要求を表す。
type Registry struct {
	SchemaVersion int           `yaml:"schema_version" json:"schema_version"`
	Requirements  []Requirement `yaml:"requirements" json:"requirements"`
}

// Requirement は一つの品質要求の宣言を表す。
type Requirement struct {
	ID             string        `yaml:"id" json:"id"`
	Name           string        `yaml:"name" json:"name"`
	Domain         string        `yaml:"domain" json:"domain"`
	Characteristic string        `yaml:"characteristic" json:"characteristic"`
	Document       Document      `yaml:"document" json:"document"`
	Applicability  Applicability `yaml:"applicability" json:"applicability"`
	Verification   Verification  `yaml:"verification" json:"verification"`
}

// Document は品質要求の本文を記述したファイルとセクションを指す。
type Document struct {
	Path    string `yaml:"path" json:"path"`
	Section string `yaml:"section" json:"section"`
}

// Applicability はチケットに品質要求を適用する条件を表す。
type Applicability struct {
	Mode       string   `yaml:"mode" json:"mode"`
	Attributes []string `yaml:"attributes,omitempty" json:"attributes,omitempty"`
}

// Verification は品質要求に記載された検証方法を表す。
type Verification struct {
	Type   string `yaml:"type" json:"type"`
	Method string `yaml:"method" json:"method"`
}

// ParseRegistry はrequirements.ymlを読み込み、宣言バージョン・品質ID・各要求の必須項目を検証する。
func ParseRegistry(data []byte) (Registry, error) {
	var result Registry
	if err := decodeDeclaration(data, &result); err != nil {
		return result, fmt.Errorf("requirements.yml: %w", err)
	}
	if result.SchemaVersion != 1 {
		return result, errors.New("requirements.yml: schema_versionは1だけに対応します")
	}
	if len(result.Requirements) == 0 {
		return result, errors.New("requirements.yml: requirementsには一つ以上の品質要求が必要です")
	}
	seen := make(map[string]bool)
	for _, req := range result.Requirements {
		if !ValidID(req.ID) || seen[req.ID] {
			return result, fmt.Errorf("requirements.yml: 品質IDが不正または重複しています: %q", req.ID)
		}
		seen[req.ID] = true
		if err := validateRequirement(req); err != nil {
			return result, fmt.Errorf("requirements.yml: %s: %w", req.ID, err)
		}
	}
	return result, nil
}

// validateRequirement は一つの品質要求について、参照先・適用方法・検証方法の指定を確認する。
func validateRequirement(req Requirement) error {
	for _, field := range []struct{ name, value string }{
		{"name", req.Name}, {"domain", req.Domain}, {"characteristic", req.Characteristic},
		{"document.path", req.Document.Path}, {"document.section", req.Document.Section},
		{"verification.method", req.Verification.Method},
	} {
		if err := ValidateText(field.value, field.name); err != nil {
			return err
		}
	}
	if !filepath.IsLocal(req.Document.Path) || filepath.Ext(req.Document.Path) != ".md" {
		return errors.New("document.pathにはプロジェクト内のMarkdown相対パスを指定してください")
	}
	if !ValidID(req.Document.Section) {
		return errors.New("document.sectionに品質IDを指定してください")
	}
	if req.Verification.Type != "automated" {
		return errors.New("verification.typeはautomatedだけに対応します")
	}
	if err := validateList(req.Applicability.Attributes, "applicability.attributes"); err != nil {
		return err
	}
	switch req.Applicability.Mode {
	case "always", "explicit":
		if len(req.Applicability.Attributes) != 0 {
			return errors.New("attributesはautomaticでだけ指定できます")
		}
	case "automatic":
		if len(req.Applicability.Attributes) != 1 {
			return errors.New("automaticのattributesは単一属性だけに対応します")
		}
	default:
		return errors.New("applicability.modeはalways・automatic・explicitから指定してください")
	}
	return nil
}
