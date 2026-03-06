package core

import (
	"maaend-client/client"
)

// CapabilitiesBuilder 设备能力构建器
type CapabilitiesBuilder struct {
	pi   *ProjectInterface
	lang string
}

// NewCapabilitiesBuilder 创建能力构建器
func NewCapabilitiesBuilder(pi *ProjectInterface, lang string) *CapabilitiesBuilder {
	if lang == "" {
		lang = "zh_cn"
	}
	return &CapabilitiesBuilder{
		pi:   pi,
		lang: lang,
	}
}

// Build 构建设备能力
func (b *CapabilitiesBuilder) Build() *client.CapabilitiesPayload {
	capabilities := &client.CapabilitiesPayload{
		Controllers: b.pi.GetControllerNames(),
		Resources:   b.pi.GetResourceNames(),
		Tasks:       make([]client.TaskInfo, 0, len(b.pi.Tasks)),
	}

	for _, task := range b.pi.Tasks {
		taskInfo := client.TaskInfo{
			Name:        task.Name,
			Label:       b.pi.GetI18nString(task.Label, b.lang),
			Description: b.pi.GetI18nString(task.Description, b.lang),
			Options:     b.buildTaskOptions(task.Option),
			Controller:  task.Controller,
			Resource:    task.Resource,
		}
		capabilities.Tasks = append(capabilities.Tasks, taskInfo)
	}

	// 构建预设信息
	for _, preset := range b.pi.Presets {
		presetInfo := client.PresetInfo{
			Name:        preset.Name,
			Label:       b.pi.GetI18nString(preset.Label, b.lang),
			Description: b.pi.GetI18nString(preset.Description, b.lang),
			Tasks:       make([]client.PresetTaskInfo, 0, len(preset.Tasks)),
		}
		for _, pt := range preset.Tasks {
			enabled := true
			if pt.Enabled != nil {
				enabled = *pt.Enabled
			}
			presetInfo.Tasks = append(presetInfo.Tasks, client.PresetTaskInfo{
				Name:    pt.Name,
				Enabled: enabled,
				Options: pt.Option,
			})
		}
		capabilities.Presets = append(capabilities.Presets, presetInfo)
	}

	return capabilities
}

// buildTaskOptions 构建任务选项
func (b *CapabilitiesBuilder) buildTaskOptions(optionNames []string) []client.OptionInfo {
	var options []client.OptionInfo

	for _, optName := range optionNames {
		opt := b.pi.GetOption(optName)
		if opt == nil {
			continue
		}

		optInfo := client.OptionInfo{
			Name:        optName,
			Type:        opt.Type,
			Label:       b.pi.GetI18nString(opt.Label, b.lang),
			Description: b.pi.GetI18nString(opt.Description, b.lang),
			DefaultCase: getDefaultCaseList(opt),
			Controller:  opt.Controller,
			Resource:    opt.Resource,
		}

		if len(opt.Cases) > 0 {
			optInfo.Cases = make([]client.CaseInfo, 0, len(opt.Cases))
			for _, c := range opt.Cases {
				optInfo.Cases = append(optInfo.Cases, client.CaseInfo{
					Name:  c.Name,
					Label: b.pi.GetI18nString(c.Label, b.lang),
				})
			}
		}

		if len(opt.Inputs) > 0 {
			optInfo.Inputs = make([]client.InputInfo, 0, len(opt.Inputs))
			for _, inp := range opt.Inputs {
				optInfo.Inputs = append(optInfo.Inputs, client.InputInfo{
					Name:         inp.Name,
					Label:        b.pi.GetI18nString(inp.Label, b.lang),
					Description:  b.pi.GetI18nString(inp.Description, b.lang),
					PipelineType: inp.PipelineType,
					Default:      inp.Default,
					Verify:       inp.Verify,
					PatternMsg:   b.pi.GetI18nString(inp.PatternMsg, b.lang),
				})
			}
		}

		options = append(options, optInfo)
	}

	return options
}
