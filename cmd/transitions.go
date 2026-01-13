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
	return spec, true
}

func (m *model) resourceTransitionScreenBackward() (fsm.State, bool) {
	if m.entity.Data.selectedGvr.Namespaced {
		return namespace, true
	}
	return gvr, true
}

// Spec Transitions
func (m *model) specGetData() (fsm.State, bool)                  { return spec, false }
func (m *model) specTransitionScreenForward() (fsm.State, bool)  { return spec, false }
func (m *model) specTransitionScreenBackward() (fsm.State, bool) { return resource, true }
