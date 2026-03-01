package runtime

func (r *Runtime) DequeueActions() []ControlAction {
	if r == nil || r.controlQ == nil {
		return nil
	}
	return r.controlQ.DequeueAll()
}

