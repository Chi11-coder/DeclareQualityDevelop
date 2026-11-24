package dqd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var identifier = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// ValidID はIDが英字で始まる英数字・アンダースコア・ハイフンで構成されているか確認する。
func ValidID(id string) bool {
	return identifier.MatchString(id)
}

// decodeDeclaration はYAMLの型・未知フィールド・重複キー・複数文書を検証して宣言を読み込む。
func decodeDeclaration(data []byte, target any) error {
	var tree yaml.Node
	if err := yaml.Unmarshal(data, &tree); err != nil {
		return err
	}
	if err := checkYAMLScalars(&tree, ""); err != nil {
		return err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return errors.New("YAML文書は一つだけ記述してください")
	}
	return nil
}

// checkYAMLScalars はYAMLの各要素を再帰的に確認し、型違いと未対応のエイリアスを拒否する。
func checkYAMLScalars(node *yaml.Node, field string) error {
	if node.Kind == yaml.AliasNode {
		return fmt.Errorf("YAMLのエイリアスは未対応です（行%d）", node.Line)
	}
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
				return fmt.Errorf("YAMLキーは文字列にしてください（行%d）", key.Line)
			}
			if err := checkYAMLScalars(value, key.Value); err != nil {
				return err
			}
		}
		return nil
	}
	if node.Kind == yaml.ScalarNode {
		want := "!!str"
		if field == "schema_version" {
			want = "!!int"
		}
		if node.Tag != want {
			return fmt.Errorf("%sの型が不正です（行%d）", field, node.Line)
		}
	}
	for _, child := range node.Content {
		if err := checkYAMLScalars(child, field); err != nil {
			return err
		}
	}
	return nil
}

// ValidateText は値が空ではなく、改行を含まない1行の文字列であることを確認する。
func ValidateText(value, name string) error {
	if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%sには空でない1行の文字列が必要です", name)
	}
	return nil
}

// validateList は文字列配列に空要素・前後の空白・重複がないことを確認する。
func validateList(values []string, name string) error {
	seen := make(map[string]bool)
	for _, value := range values {
		if err := ValidateText(value, name); err != nil {
			return err
		}
		if strings.TrimSpace(value) != value || seen[value] {
			return fmt.Errorf("%sに前後の空白または重複があります: %q", name, value)
		}
		seen[value] = true
	}
	return nil
}
