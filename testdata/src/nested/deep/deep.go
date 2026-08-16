// Package deep sits under a nested directory so the fixture set varies the file
// PATH: a rule keyed on where a file lives is invisible to a corpus whose files
// all sit in one directory.
package deep

// Held stores a value through the empty interface.
type Held struct {
	v interface{} // want `prefer any to the empty interface\{\}`
}

// Marker is the empty interface this package exports, so a sibling fixture can
// embed it across a package boundary — the shape whose rewrite would delete the
// import that carries it.
type Marker interface{} // want `prefer any to the empty interface\{\}`

// probeMarker is an empty interface spelt in place, so it is reported.
type probeMarker interface{} // want `prefer any to the empty interface\{\}`

// composed embeds that name. It denotes the empty interface and is not a
// spelling of it, so it is not reported HERE — in this directory, in this file
// class, under this package clause — which is what a widening keyed on any of
// those three dimensions would change.
type composed interface{ probeMarker }
