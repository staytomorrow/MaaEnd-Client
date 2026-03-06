package core

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// OptionResolver 选项解析器
type OptionResolver struct {
	pi *ProjectInterface
}

// NewOptionResolver 创建选项解析器
func NewOptionResolver(pi *ProjectInterface) *OptionResolver {
	return &OptionResolver{pi: pi}
}

// ResolveContext 选项解析上下文
type ResolveContext struct {
	Controller string
	Resource   string
}

// ResolveTaskOptions 解析任务选项，构建 pipeline_override
func (r *OptionResolver) ResolveTaskOptions(taskName string, userOptions map[string]interface{}, ctx ResolveContext) (map[string]interface{}, error) {
	task := r.pi.GetTask(taskName)
	if task == nil {
		return nil, fmt.Errorf("任务不存在: %s", taskName)
	}

	if userOptions == nil {
		userOptions = make(map[string]interface{})
	}

	// 合并 override
	override := make(map[string]interface{})

	// 首先应用任务级别的 pipeline_override
	if task.PipelineOverride != nil {
		mergeOverride(override, task.PipelineOverride)
	}

	// 标准顺序：global_option < resource.option < controller.option < task.option
	sources := [][]string{
		r.pi.GlobalOption,
	}
	if res := r.pi.GetResource(ctx.Resource); res != nil {
		sources = append(sources, res.Option)
	}
	if ctrl := r.pi.GetController(ctx.Controller); ctrl != nil {
		sources = append(sources, ctrl.Option)
	}
	sources = append(sources, task.Option)

	for _, optionNames := range sources {
		for _, optName := range optionNames {
			opt := r.pi.GetOption(optName)
			if opt == nil {
				continue
			}
			if !isOptionActive(opt, ctx.Controller, ctx.Resource) {
				continue
			}

			userValue := userOptions[optName]
			optOverride, err := r.resolveOption(optName, opt, userValue, userOptions, ctx)
			if err != nil {
				return nil, fmt.Errorf("解析选项 %s 失败: %w", optName, err)
			}

			if optOverride != nil {
				mergeOverride(override, optOverride)
			}
		}
	}

	return override, nil
}

// resolveOption 解析单个选项
func (r *OptionResolver) resolveOption(name string, opt *OptionConfig, userValue interface{}, allUserOptions map[string]interface{}, ctx ResolveContext) (map[string]interface{}, error) {
	switch opt.Type {
	case "select":
		return r.resolveSelectOption(name, opt, userValue, allUserOptions, ctx)
	case "switch":
		// switch 类型与 select 逻辑相同，只是通常只有 Yes/No 两个选项
		return r.resolveSelectOption(name, opt, userValue, allUserOptions, ctx)
	case "checkbox":
		return r.resolveCheckboxOption(name, opt, userValue, allUserOptions, ctx)
	case "input":
		return r.resolveInputOption(name, opt, userValue, allUserOptions)
	default:
		return nil, fmt.Errorf("未知选项类型: %s", opt.Type)
	}
}

// resolveSelectOption 解析选择类型选项（也用于 switch 类型）
func (r *OptionResolver) resolveSelectOption(name string, opt *OptionConfig, userValue interface{}, allUserOptions map[string]interface{}, ctx ResolveContext) (map[string]interface{}, error) {
	override := make(map[string]interface{})

	// 获取选中的 case
	// 优先级：用户选择 > DefaultCase > Default > 第一个 case
	selectedCase := getDefaultCaseFirst(opt)
	if selectedCase == "" && opt.Default != "" {
		selectedCase = opt.Default
	}
	if userValue != nil {
		if strVal, ok := userValue.(string); ok {
			selectedCase = strVal
		}
	}

	// 如果仍然为空，使用第一个 case 作为默认
	if selectedCase == "" && len(opt.Cases) > 0 {
		selectedCase = opt.Cases[0].Name
	}

	// 查找对应的 case
	var caseConfig *CaseConfig
	for i := range opt.Cases {
		if opt.Cases[i].Name == selectedCase {
			caseConfig = &opt.Cases[i]
			break
		}
	}

	if caseConfig == nil {
		// 如果没有找到且有 cases，返回空 override 而非错误
		if len(opt.Cases) == 0 {
			return override, nil
		}
		return nil, fmt.Errorf("选项 %s 的 case %s 不存在", name, selectedCase)
	}

	// 应用 case 的 pipeline_override
	if caseConfig.PipelineOverride != nil {
		mergeOverride(override, caseConfig.PipelineOverride)
	}

	// 递归处理嵌套选项
	for _, nestedOptName := range caseConfig.Option {
		nestedOpt := r.pi.GetOption(nestedOptName)
		if nestedOpt == nil {
			continue
		}
		if !isOptionActive(nestedOpt, ctx.Controller, ctx.Resource) {
			continue
		}

		nestedValue := allUserOptions[nestedOptName]
		nestedOverride, err := r.resolveOption(nestedOptName, nestedOpt, nestedValue, allUserOptions, ctx)
		if err != nil {
			return nil, fmt.Errorf("解析嵌套选项 %s 失败: %w", nestedOptName, err)
		}

		if nestedOverride != nil {
			mergeOverride(override, nestedOverride)
		}
	}

	return override, nil
}

// resolveCheckboxOption 解析复选框类型选项
func (r *OptionResolver) resolveCheckboxOption(_ string, opt *OptionConfig, userValue interface{}, allUserOptions map[string]interface{}, ctx ResolveContext) (map[string]interface{}, error) {
	override := make(map[string]interface{})

	// 获取选中的 cases
	selectedSet := make(map[string]struct{})
	if userValue != nil {
		switch v := userValue.(type) {
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok {
					if name := strings.TrimSpace(str); name != "" {
						selectedSet[name] = struct{}{}
					}
				}
			}
		case []string:
			for _, item := range v {
				if name := strings.TrimSpace(item); name != "" {
					selectedSet[name] = struct{}{}
				}
			}
		}
	}

	// 如果没有选择，使用默认值
	if len(selectedSet) == 0 {
		for _, name := range getDefaultCaseList(opt) {
			selectedSet[name] = struct{}{}
		}
	}

	// 按 cases 定义顺序合并，符合 PI v2.3.0 规范
	for i := range opt.Cases {
		caseConfig := &opt.Cases[i]
		if _, selected := selectedSet[caseConfig.Name]; !selected {
			continue
		}

		// 应用 case 的 pipeline_override
		if caseConfig.PipelineOverride != nil {
			mergeOverride(override, caseConfig.PipelineOverride)
		}

		// 递归处理嵌套选项
		for _, nestedOptName := range caseConfig.Option {
			nestedOpt := r.pi.GetOption(nestedOptName)
			if nestedOpt == nil {
				continue
			}
			if !isOptionActive(nestedOpt, ctx.Controller, ctx.Resource) {
				continue
			}

			nestedValue := allUserOptions[nestedOptName]
			nestedOverride, err := r.resolveOption(nestedOptName, nestedOpt, nestedValue, allUserOptions, ctx)
			if err != nil {
				return nil, fmt.Errorf("解析嵌套选项 %s 失败: %w", nestedOptName, err)
			}

			if nestedOverride != nil {
				mergeOverride(override, nestedOverride)
			}
		}
	}

	return override, nil
}

func isOptionActive(opt *OptionConfig, controller, resource string) bool {
	if opt == nil {
		return false
	}

	if len(opt.Controller) > 0 && !containsString(opt.Controller, controller) {
		return false
	}
	if len(opt.Resource) > 0 && !containsString(opt.Resource, resource) {
		return false
	}

	return true
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

// resolveInputOption 解析输入类型选项
func (r *OptionResolver) resolveInputOption(_ string, opt *OptionConfig, userValue interface{}, _ map[string]interface{}) (map[string]interface{}, error) {
	override := make(map[string]interface{})

	// 获取输入值
	inputValues := make(map[string]string)

	// 从 inputs 配置获取默认值
	for _, input := range opt.Inputs {
		defaultVal := input.GetDefaultString()
		if defaultVal != "" {
			inputValues[input.Name] = defaultVal
		}
	}

	// 解析输入 schema（name -> pipeline_type）
	inputTypes := make(map[string]string)
	for _, input := range opt.Inputs {
		if input.PipelineType != "" {
			inputTypes[input.Name] = input.PipelineType
		}
	}

	// 覆盖用户输入
	if userValue != nil {
		switch v := userValue.(type) {
		case map[string]interface{}:
			for k, val := range v {
				inputValues[k] = formatInputValue(val)
			}
		case map[string]string:
			for k, val := range v {
				inputValues[k] = val
			}
		}
	}

	// 应用 pipeline_override 并进行变量替换
	if opt.PipelineOverride != nil {
		resolved := resolveVariables(opt.PipelineOverride, inputValues, inputTypes)
		mergeOverride(override, resolved)
	}

	return override, nil
}

// resolveVariables 替换 pipeline_override 中的变量
func resolveVariables(override map[string]interface{}, values map[string]string, inputTypes map[string]string) map[string]interface{} {
	// 深拷贝
	data, _ := json.Marshal(override)
	var result map[string]interface{}
	json.Unmarshal(data, &result)

	// 递归替换
	resolveVariablesRecursive(result, values, inputTypes)

	return result
}

// resolveVariablesRecursive 递归替换变量
func resolveVariablesRecursive(data interface{}, values map[string]string, inputTypes map[string]string) {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, val := range v {
			switch vv := val.(type) {
			case string:
				v[key] = resolveStringValue(vv, values, inputTypes)
			case map[string]interface{}:
				resolveVariablesRecursive(vv, values, inputTypes)
			case []interface{}:
				resolveVariablesRecursive(vv, values, inputTypes)
			}
		}
	case []interface{}:
		for i, item := range v {
			switch vv := item.(type) {
			case string:
				v[i] = resolveStringValue(vv, values, inputTypes)
			case map[string]interface{}:
				resolveVariablesRecursive(vv, values, inputTypes)
			case []interface{}:
				resolveVariablesRecursive(vv, values, inputTypes)
			}
		}
	}
}

func resolveStringValue(s string, values map[string]string, inputTypes map[string]string) interface{} {
	exactVar := extractExactVariable(s)
	if exactVar == "" {
		return replaceVariables(s, values)
	}

	raw, ok := values[exactVar]
	if !ok {
		return s
	}

	typ := strings.ToLower(strings.TrimSpace(inputTypes[exactVar]))
	converted, ok := convertByPipelineType(raw, typ)
	if !ok {
		return raw
	}
	return converted
}

func extractExactVariable(s string) string {
	re := regexp.MustCompile(`^\{(\w+)\}$`)
	matches := re.FindStringSubmatch(strings.TrimSpace(s))
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

func convertByPipelineType(raw, pipelineType string) (interface{}, bool) {
	switch pipelineType {
	case "", "string":
		return raw, true
	case "int":
		iv, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil {
			return nil, false
		}
		return iv, true
	case "bool":
		bv, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return nil, false
		}
		return bv, true
	default:
		return raw, true
	}
}

// replaceVariables 替换字符串中的变量
func replaceVariables(s string, values map[string]string) string {
	// 匹配 {varName} 格式
	re := regexp.MustCompile(`\{(\w+)\}`)
	return re.ReplaceAllStringFunc(s, func(match string) string {
		varName := match[1 : len(match)-1]
		if val, ok := values[varName]; ok {
			return val
		}
		return match
	})
}

// mergeOverride 合并 pipeline_override
func mergeOverride(dst, src map[string]interface{}) {
	for key, srcVal := range src {
		if dstVal, exists := dst[key]; exists {
			// 如果都是 map，递归合并
			if dstMap, ok := dstVal.(map[string]interface{}); ok {
				if srcMap, ok := srcVal.(map[string]interface{}); ok {
					mergeOverride(dstMap, srcMap)
					continue
				}
			}
		}
		// 否则直接覆盖
		dst[key] = srcVal
	}
}

// formatInputValue 格式化输入值为字符串
func formatInputValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		// JSON 数字解析为 float64
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case int:
		return fmt.Sprintf("%d", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", v)
	}
}
