package vascula

import "encoding/json"

// loopInfo is forloop: the position of the current iteration. Its properties
// are index, index0, rindex, rindex0, first, last, length and parentloop (the
// enclosing loop's forloop, nil at the outermost loop).
type loopInfo struct {
	index, length int
	parent        *loopInfo
}

func (l *loopInfo) field(name string) interface{} {
	switch name {
	case "index":
		return float64(l.index + 1)
	case "index0":
		return float64(l.index)
	case "rindex":
		return float64(l.length - l.index)
	case "rindex0":
		return float64(l.length - l.index - 1)
	case "first":
		return l.index == 0
	case "last":
		return l.index == l.length-1
	case "length":
		return float64(l.length)
	case "size":
		return float64(len(loopFields))
	case "parentloop":
		if l.parent == nil {
			return nil
		}
		return l.parent
	}
	return nil
}

var loopFields = []string{"first", "index", "index0", "last", "length", "parentloop", "rindex", "rindex0"}

// asMap is forloop as the plain object it describes, for json and iteration.
func (l *loopInfo) asMap() map[string]interface{} {
	m := make(map[string]interface{}, len(loopFields))
	for _, name := range loopFields {
		m[name] = l.field(name)
	}
	if l.parent != nil {
		m["parentloop"] = l.parent.asMap()
	}
	return m
}

func (l *loopInfo) MarshalJSON() ([]byte, error) { return json.Marshal(l.asMap()) }
