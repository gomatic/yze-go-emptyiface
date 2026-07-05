package a

// Handler uses the empty interface and must be flagged (and fixed to any).
type Handler interface{} // want `prefer any`

// accept takes an empty interface parameter and must be flagged.
func accept(x interface{}) { _ = x } // want `prefer any`

// Real has methods and must not be flagged.
type Real interface {
	Do()
}

// Already uses any directly and must never be flagged (regression guard).
type Already any

// acceptAny uses any directly and must never be flagged (regression guard).
func acceptAny(x any) { _ = x }

// Constraint is a non-empty type constraint (~int) and must not be flagged.
type Constraint interface{ ~int }

// identity carries an empty type constraint that must be flagged and fixed to any.
func identity[T interface{}](v T) T { return v } // want `prefer any`

// Bag stores an empty interface field that must be flagged.
type Bag struct {
	v interface{} // want `prefer any`
}

// registry maps to the empty interface and must be flagged.
var registry = map[string]interface{}{} // want `prefer any`

// shadowed declares a local alias named any, so rewriting interface{} to any
// would change its meaning; the diagnostic still fires but no fix is offered.
func shadowed() {
	type any = int
	var x interface{} // want `prefer any`
	_ = x
}

// commented carries a comment inside the braces that the rewrite would destroy;
// the diagnostic still fires but no fix is offered.
var commented interface { // want `prefer any`
	// sacred note that the fix must not destroy
}
