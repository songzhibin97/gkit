package parse_pb

// Only value edges can form an invalid recursive Go type. Slices, maps and
// pointers already provide indirection, so their existing field APIs stay intact.
func (p *PbParseGo) breakRecursiveValues() {
	edges := make(map[string][]string)
	for name, message := range p.Message {
		for _, field := range message.Files {
			if _, ok := p.Message[field.TypeGo]; ok {
				edges[name] = append(edges[name], field.TypeGo)
			}
		}
	}
	var reaches func(string, string, map[string]bool) bool
	reaches = func(from, target string, visited map[string]bool) bool {
		if from == target {
			return true
		}
		if visited[from] {
			return false
		}
		visited[from] = true
		for _, next := range edges[from] {
			if reaches(next, target, visited) {
				return true
			}
		}
		return false
	}
	for name, message := range p.Message {
		for _, field := range message.Files {
			if _, ok := p.Message[field.TypeGo]; ok && reaches(field.TypeGo, name, make(map[string]bool)) {
				field.TypeGo = "*" + field.TypeGo
			}
		}
	}
}
