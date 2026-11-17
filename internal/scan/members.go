package scan

func ObjectMembers(b []byte, i, budget int) (count, end int) {
	limit := i + budget
	for i < len(b) && b[i] == '"' {
		strEnd, _, ok := StringEnd(b, i+1)
		if !ok {
			return count, strEnd
		}
		i = SkipSpace(b, strEnd+1)
		if i >= len(b) || b[i] != ':' {
			return count, i
		}
		if i, ok = ValueEnd(b, SkipSpace(b, i+1), 0); !ok {
			return count, i
		}
		count++
		i = SkipSpace(b, i)
		if i >= len(b) || b[i] != ',' || i > limit {
			return count, i
		}
		i = SkipSpace(b, i+1)
	}
	return count, i
}
