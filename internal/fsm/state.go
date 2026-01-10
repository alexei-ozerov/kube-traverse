package fsm

type State int
type Event int

type StateFn func() (State, bool)

type Entity struct {
	initialState State
	currentState State
	machine      [][]StateFn
}

func (entity *Entity) SetInitialState(initialState State) {
	entity.initialState = initialState
	entity.currentState = initialState
}

func (entity *Entity) SetMachine(m [][]StateFn) {
	entity.machine = m
}

func (entity *Entity) GetCurrentState() State {
	return entity.currentState
}

func (entity *Entity) GetInitialState() State {
	return entity.initialState
}

func (entity *Entity) Dispatch(e Event) {
	fn := entity.machine[entity.currentState][e]
	next, transition := fn()

	// If the given currentState and event combination result in a transition
	// apply the transition to the Entity structure
	if transition {
		entity.currentState = next
	}
}
