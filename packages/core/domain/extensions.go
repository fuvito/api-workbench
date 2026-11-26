package domain

// Extensions holds the x- keys an author put on one item of a file, which a save writes back
// (B-153). It is one line per key, in file order: the key as a JSON string, ": ", and the value as
// compact JSON. It is text so that the types stay comparable and the editors can carry it with its
// item without reading it (JavaScript would round large numbers).
type Extensions string
