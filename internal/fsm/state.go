package fsm

type State int
type Event int

type StateFn func() (State, bool)

type Entity[T any] struct {
	Data *T

	initialState State
	currentState State
	machine      [][]StateFn
}

func (entity *Entity[T]) SetInitialState(initialState State) {
	entity.initialState = initialState
	entity.currentState = initialState
}

func (entity *Entity[T]) SetMachine(m [][]StateFn) {
	entity.machine = m
}

func (entity *Entity[T]) GetCurrentState() State {
	return entity.currentState
}

func (entity *Entity[T]) GetInitialState() State {
	return entity.initialState
}

func (entity *Entity[T]) Dispatch(e Event) {
	fn := entity.machine[entity.currentState][e]
	next, transition := fn()

	// If the given currentState and event combination result in a transition
	// apply the transition to the Entity structure
	if transition {
		entity.currentState = next
	}
}
