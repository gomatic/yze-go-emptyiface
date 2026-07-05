package a

// wrapped lives in a second file so the analyzer must locate the correct
// enclosing file when scanning comments; it must be flagged and fixed to any.
func wrapped(v interface{}) any { return v } // want `prefer any`
