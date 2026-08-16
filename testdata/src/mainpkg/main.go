// Command mainpkg varies the PACKAGE CLAUSE: package main is a file class of its
// own to anything reasoning about what an importer can reach, and a rule keyed
// on the clause is invisible to a corpus whose packages are all libraries.
package main

// sink accepts anything, spelt the long way.
func sink(v interface{}) { _ = v } // want `prefer any to the empty interface\{\}`

func main() { sink(1) }

// mainMarker is an empty interface spelt in place, so it is reported.
type mainMarker interface{} // want `prefer any to the empty interface\{\}`

// composedInMain embeds that name under a main package clause, and is not
// reported: a widening keyed on the clause fails here.
type composedInMain interface{ mainMarker }
