package core

import (
	"fmt"
	"testing"
)

func TestLoadInterface(t *testing.T) {
	// 测试加载 MaaEnd v1.6.5 的 interface.json
	maaEndPath := "E:\\Dnyo\\Documents\\work\\endfield\\Endfield-API\\MaaEnd-win-x86_64-v1.6.5"

	pi, err := LoadInterface(maaEndPath)
	if err != nil {
		t.Fatalf("加载 interface.json 失败: %v", err)
	}

	// 验证基本信息
	fmt.Printf("项目名称: %s\n", pi.Name)
	fmt.Printf("项目版本: %s\n", pi.Version)
	fmt.Printf("Import 文件数: %d\n", len(pi.Import))

	// 验证任务是否正确加载
	fmt.Printf("加载的任务数: %d\n", len(pi.Tasks))
	if len(pi.Tasks) == 0 {
		t.Error("任务列表为空，import 可能没有正确工作")
	}

	// 打印所有任务
	fmt.Println("\n=== 任务列表 ===")
	for _, task := range pi.Tasks {
		fmt.Printf("- %s (%s)\n", task.Name, pi.GetI18nString(task.Label, "zh_cn"))
		fmt.Printf("  选项: %v\n", task.Option)
		fmt.Printf("  控制器: %v\n", task.Controller)
	}

	// 验证选项是否正确加载
	fmt.Printf("\n加载的选项数: %d\n", len(pi.Options))
	if len(pi.Options) == 0 {
		t.Error("选项列表为空，import 可能没有正确工作")
	}

	// 打印部分选项
	fmt.Println("\n=== 部分选项（前 5 个） ===")
	count := 0
	for name, opt := range pi.Options {
		if count >= 5 {
			break
		}
		fmt.Printf("- %s (类型: %s)\n", name, opt.Type)
		count++
	}

	// 验证 switch 类型选项
	fmt.Println("\n=== switch 类型选项 ===")
	for name, opt := range pi.Options {
		if opt.Type == "switch" {
			fmt.Printf("- %s: %d cases\n", name, len(opt.Cases))
			if len(opt.Cases) > 0 {
				fmt.Printf("  第一个 case: %s\n", opt.Cases[0].Name)
			}
			break
		}
	}

	// 验证控制器
	fmt.Printf("\n控制器数: %d\n", len(pi.Controllers))
	for _, ctrl := range pi.Controllers {
		fmt.Printf("- %s (类型: %s)\n", ctrl.Name, ctrl.Type)
	}

	// 验证 Preset
	fmt.Printf("\n预设数: %d\n", len(pi.Presets))
	for _, preset := range pi.Presets {
		fmt.Printf("- %s (%s): %d 个任务\n", preset.Name, pi.GetI18nString(preset.Label, "zh_cn"), len(preset.Tasks))
		for _, pt := range preset.Tasks {
			fmt.Printf("    任务: %s, 选项: %v\n", pt.Name, pt.Option)
		}
	}
	if len(pi.Presets) == 0 {
		t.Error("预设列表为空，preset 导入可能没有正确工作")
	}

	// 验证 Agent 数组
	fmt.Printf("\nAgent 数: %d\n", len(pi.Agent))
	for _, agent := range pi.Agent {
		fmt.Printf("- exec: %s, args: %v\n", agent.ChildExec, agent.ChildArgs)
	}
	if len(pi.Agent) == 0 {
		t.Error("Agent 列表为空")
	}

	// 验证 input 类型选项的 Inputs 字段
	fmt.Println("\n=== input 类型选项 ===")
	for name, opt := range pi.Options {
		if opt.Type == "input" && len(opt.Inputs) > 0 {
			fmt.Printf("- %s: %d 个输入字段\n", name, len(opt.Inputs))
			for _, inp := range opt.Inputs {
				fmt.Printf("    %s (type=%s, default=%v, verify=%s)\n", inp.Name, inp.PipelineType, inp.Default, inp.Verify)
			}
			break
		}
	}

	// 验证 AttachResourcePath
	for _, ctrl := range pi.Controllers {
		if len(ctrl.AttachResourcePath) > 0 {
			fmt.Printf("\n控制器 %s 附加资源: %v\n", ctrl.Name, ctrl.AttachResourcePath)
		}
	}

	// 验证 ResourceConfig.Label
	fmt.Println("\n=== 资源 ===")
	for _, res := range pi.Resources {
		fmt.Printf("- %s (label: %s)\n", res.Name, pi.GetI18nString(res.Label, "zh_cn"))
	}
}
