package main

import (
	"github.com/alexei-ozerov/kube-traverse/internal/fsm"
)

/*
State Transitions
*/

// GVR Transitions
func (m *model) gvrTransitionScreenForward() (fsm.State, bool) {
	if m.entity.Data.selectedGvr.Namespaced {
		return namespace, true
	}
	return resource, true
}

func (m *model) gvrTransitionScreenBackward() (fsm.State, bool) {
	return gvr, false
}

// Namespace Transitions
func (m *model) namespaceTransitionScreenForward() (fsm.State, bool) {
	return resource, true
}

func (m *model) namespaceTransitionScreenBackward() (fsm.State, bool) {
	return gvr, true
}

// Resource Transitions
func (m *model) resourceTransitionScreenForward() (fsm.State, bool) {
	return action, true
}

func (m *model) resourceTransitionScreenBackward() (fsm.State, bool) {
	if m.entity.Data.selectedGvr.Namespaced {
		return namespace, true
	}
	return gvr, true
}

// Action Transitions
func (m *model) actionTransitionScreenForward() (fsm.State, bool) {
	if m.entity.Data.choice == "spec" {
		return spec, true
	}

	if m.entity.Data.choice == "log" {
		return container, true
	}

	return action, false
}
func (m *model) actionTransitionScreenBackward() (fsm.State, bool) { return resource, true }

// Spec Transitions
func (m *model) specTransitionScreenForward() (fsm.State, bool) { return spec, false }
func (m *model) specTransitionScreenBackward() (fsm.State, bool) {
	return action, true
}

// Container Transitios
func (m *model) containerTransitionScreenForward() (fsm.State, bool) {
	return logs, true
}
func (m *model) containerTransitionScreenBackward() (fsm.State, bool) {
	return action, true
}

// Logs Transitions
func (m *model) logsTransitionScreenForward() (fsm.State, bool) { return logs, false }
func (m *model) logsTransitionScreenBackward() (fsm.State, bool) {
	if m.entity.Data.cancelLog != nil {
		m.entity.Data.cancelLog()
	}
	return container, true
}
