package runtime

// Context: This file belongs to the MetricsNode application layer around control_actions.

func (r *Runtime) DequeueActions() []ControlAction {
	if r == nil || r.controlQ == nil {
		return nil
	}
	return r.controlQ.DequeueAll()
}
