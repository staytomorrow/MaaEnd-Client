package core

import "fmt"

// CompileErrorCode 编译错误码
type CompileErrorCode string

const (
	CompileErrTaskNotFound      CompileErrorCode = "task_not_found"
	CompileErrOptionResolveFail CompileErrorCode = "option_resolve_failed"
)

// CompileError 任务参数编译错误
type CompileError struct {
	Code     CompileErrorCode
	TaskName string
	Cause    error
}

func (e *CompileError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("任务参数编译失败[%s]: %s", e.Code, e.TaskName)
	}
	return fmt.Sprintf("任务参数编译失败[%s]: %s: %v", e.Code, e.TaskName, e.Cause)
}

func (e *CompileError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// CompiledTask 编译后的任务
type CompiledTask struct {
	Task     *TaskConfig
	Override map[string]interface{}
}

// TaskCompiler 任务参数编译器
type TaskCompiler struct {
	pi       *ProjectInterface
	resolver *OptionResolver
}

// NewTaskCompiler 创建任务参数编译器
func NewTaskCompiler(pi *ProjectInterface) *TaskCompiler {
	return &TaskCompiler{
		pi:       pi,
		resolver: NewOptionResolver(pi),
	}
}

// Compile 编译单个任务的最终 pipeline_override
func (c *TaskCompiler) Compile(taskName string, userOptions map[string]interface{}, ctx ResolveContext) (*CompiledTask, error) {
	task := c.pi.GetTask(taskName)
	if task == nil {
		return nil, &CompileError{Code: CompileErrTaskNotFound, TaskName: taskName}
	}

	override, err := c.resolver.ResolveTaskOptions(taskName, userOptions, ctx)
	if err != nil {
		return nil, &CompileError{Code: CompileErrOptionResolveFail, TaskName: taskName, Cause: err}
	}

	return &CompiledTask{
		Task:     task,
		Override: override,
	}, nil
}
