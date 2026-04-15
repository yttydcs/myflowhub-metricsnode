package runtime

// 本文件承载 MetricsNode 应用层中与 `control_actions` 相关的逻辑。

func (r *Runtime) DequeueActions() []ControlAction {
	if r == nil || r.controlQ == nil {
		return nil
	}
	return r.controlQ.DequeueAll()
}
