package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTaskOptions_PIV231MergeOrderAndActivation(t *testing.T) {
	pi := &ProjectInterface{
		GlobalOption: []string{"globalOpt", "inactiveGlobal"},
		Controllers: []ControllerConfig{
			{
				Name:   "ctrlA",
				Option: []string{"controllerOpt"},
			},
		},
		Resources: []ResourceConfig{
			{
				Name:   "resA",
				Option: []string{"resourceOpt"},
			},
		},
		Tasks: []TaskConfig{
			{
				Name:   "taskA",
				Option: []string{"taskOpt"},
			},
		},
		Options: map[string]*OptionConfig{
			"globalOpt": {
				Type: "switch",
				Cases: []CaseConfig{{Name: "on", PipelineOverride: map[string]interface{}{
					"Node": map[string]interface{}{"value": "global"},
				}}},
				DefaultCase: "on",
			},
			"inactiveGlobal": {
				Type:       "switch",
				Controller: []string{"ctrlB"},
				Cases: []CaseConfig{{Name: "on", PipelineOverride: map[string]interface{}{
					"Node": map[string]interface{}{"inactive": true},
				}}},
				DefaultCase: "on",
			},
			"resourceOpt": {
				Type: "switch",
				Cases: []CaseConfig{{Name: "on", PipelineOverride: map[string]interface{}{
					"Node": map[string]interface{}{"value": "resource"},
				}}},
				DefaultCase: "on",
			},
			"controllerOpt": {
				Type: "switch",
				Cases: []CaseConfig{{Name: "on", PipelineOverride: map[string]interface{}{
					"Node": map[string]interface{}{"value": "controller"},
				}}},
				DefaultCase: "on",
			},
			"taskOpt": {
				Type: "switch",
				Cases: []CaseConfig{{
					Name:   "on",
					Option: []string{"nestedActive", "nestedInactive"},
					PipelineOverride: map[string]interface{}{
						"Node": map[string]interface{}{"value": "task"},
					},
				}},
				DefaultCase: "on",
			},
			"nestedActive": {
				Type: "switch",
				Cases: []CaseConfig{{Name: "on", PipelineOverride: map[string]interface{}{
					"Node": map[string]interface{}{"nested": "active"},
				}}},
				DefaultCase: "on",
			},
			"nestedInactive": {
				Type:     "switch",
				Resource: []string{"resB"},
				Cases: []CaseConfig{{Name: "on", PipelineOverride: map[string]interface{}{
					"Node": map[string]interface{}{"nested": "inactive"},
				}}},
				DefaultCase: "on",
			},
		},
	}

	resolver := NewOptionResolver(pi)
	override, err := resolver.ResolveTaskOptions("taskA", nil, ResolveContext{Controller: "ctrlA", Resource: "resA"})
	if err != nil {
		t.Fatalf("ResolveTaskOptions failed: %v", err)
	}

	nodeAny, ok := override["Node"]
	if !ok {
		t.Fatalf("missing Node in override")
	}
	node, ok := nodeAny.(map[string]interface{})
	if !ok {
		t.Fatalf("Node is not object")
	}

	if got := node["value"]; got != "task" {
		t.Fatalf("expected task to win merge order, got %v", got)
	}
	if got := node["nested"]; got != "active" {
		t.Fatalf("expected nested active override, got %v", got)
	}
	if _, exists := node["inactive"]; exists {
		t.Fatalf("inactive option should not be merged")
	}
}

func TestResolveTaskOptions_CheckboxUsesCaseDefinitionOrder(t *testing.T) {
	pi := &ProjectInterface{
		Tasks: []TaskConfig{{Name: "taskA", Option: []string{"multi"}}},
		Options: map[string]*OptionConfig{
			"multi": {
				Type: "checkbox",
				Cases: []CaseConfig{
					{Name: "A", PipelineOverride: map[string]interface{}{"Node": map[string]interface{}{"order": "A"}}},
					{Name: "B", PipelineOverride: map[string]interface{}{"Node": map[string]interface{}{"order": "B"}}},
				},
			},
		},
	}

	resolver := NewOptionResolver(pi)
	override, err := resolver.ResolveTaskOptions("taskA", map[string]interface{}{"multi": []string{"B", "A"}}, ResolveContext{})
	if err != nil {
		t.Fatalf("ResolveTaskOptions failed: %v", err)
	}

	node, ok := override["Node"].(map[string]interface{})
	if !ok {
		t.Fatalf("Node is not object")
	}
	if got := node["order"]; got != "B" {
		t.Fatalf("expected case-definition-order merge result B, got %v", got)
	}
}

func TestResolveTaskOptions_InputPipelineTypeConversion(t *testing.T) {
	pi := &ProjectInterface{
		Tasks: []TaskConfig{{Name: "taskA", Option: []string{"inputOpt"}}},
		Options: map[string]*OptionConfig{
			"inputOpt": {
				Type: "input",
				Inputs: []InputConfig{
					{Name: "Count", PipelineType: "int", Default: "3"},
					{Name: "Enable", PipelineType: "bool", Default: "true"},
					{Name: "Name", PipelineType: "string", Default: "demo"},
				},
				PipelineOverride: map[string]interface{}{
					"Node": map[string]interface{}{
						"count":  "{Count}",
						"enable": "{Enable}",
						"name":   "{Name}",
						"text":   "prefix-{Count}",
					},
				},
			},
		},
	}

	resolver := NewOptionResolver(pi)
	override, err := resolver.ResolveTaskOptions("taskA", map[string]interface{}{
		"inputOpt": map[string]interface{}{
			"Count":  "42",
			"Enable": "false",
			"Name":   "custom",
		},
	}, ResolveContext{})
	if err != nil {
		t.Fatalf("ResolveTaskOptions failed: %v", err)
	}

	node, ok := override["Node"].(map[string]interface{})
	if !ok {
		t.Fatalf("Node is not object")
	}

	if got, ok := node["count"].(int); !ok || got != 42 {
		t.Fatalf("expected int count=42, got %#v", node["count"])
	}
	if got, ok := node["enable"].(bool); !ok || got {
		t.Fatalf("expected bool enable=false, got %#v", node["enable"])
	}
	if got, ok := node["name"].(string); !ok || got != "custom" {
		t.Fatalf("expected string name=custom, got %#v", node["name"])
	}
	if got := node["text"]; got != "prefix-42" {
		t.Fatalf("expected text replacement prefix-42, got %#v", got)
	}
}

func TestLoadInterface_AgentObjectAndImportMergeByName(t *testing.T) {
	dir := t.TempDir()

	mainContent := `{
		"interface_version": 2,
		"name": "demo",
		"agent": {
			"child_exec": "python",
			"child_args": ["agent.py"]
		},
		"task": [{"name": "T1", "entry": "E1", "option": []}],
		"option": {
			"optMain": {"type": "switch", "default_case": "on", "cases": [{"name": "on"}]}
		},
		"preset": [{"name": "P1", "task": [{"name": "T1", "enabled": true}]}],
		"import": ["import.json"]
	}`
	importContent := `{
		"task": [
			{"name": "T1", "entry": "E1-import", "option": []},
			{"name": "T2", "entry": "E2", "option": []}
		],
		"option": {
			"optMain": {"type": "switch", "default_case": "off", "cases": [{"name": "off"}]},
			"optImport": {"type": "switch", "default_case": "on", "cases": [{"name": "on"}]}
		},
		"preset": [
			{"name": "P1", "task": [{"name": "T2", "enabled": false}]},
			{"name": "P2", "task": [{"name": "T2"}]}
		]
	}`

	if err := os.WriteFile(filepath.Join(dir, "interface.json"), []byte(mainContent), 0644); err != nil {
		t.Fatalf("write interface.json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "import.json"), []byte(importContent), 0644); err != nil {
		t.Fatalf("write import.json failed: %v", err)
	}

	pi, err := LoadInterface(dir)
	if err != nil {
		t.Fatalf("LoadInterface failed: %v", err)
	}

	if len(pi.Agent) != 1 || pi.Agent[0].ChildExec != "python" {
		t.Fatalf("agent object should normalize to single-item list, got %#v", pi.Agent)
	}

	if len(pi.Tasks) != 2 {
		t.Fatalf("expected 2 tasks after merge, got %d", len(pi.Tasks))
	}
	if t1 := pi.GetTask("T1"); t1 == nil || t1.Entry != "E1-import" {
		t.Fatalf("task T1 should be overridden by import, got %#v", t1)
	}

	if optMain := pi.GetOption("optMain"); optMain == nil || getDefaultCaseFirst(optMain) != "off" {
		t.Fatalf("option optMain should be overridden by import")
	}

	if len(pi.Presets) != 2 {
		t.Fatalf("expected 2 presets after merge, got %d", len(pi.Presets))
	}
	if p1 := pi.GetPreset("P1"); p1 == nil || len(p1.Tasks) == 0 || p1.Tasks[0].Name != "T2" {
		t.Fatalf("preset P1 should be overridden by import, got %#v", p1)
	}
	if p2 := pi.GetPreset("P2"); p2 == nil {
		t.Fatalf("preset P2 should exist")
	}
}
