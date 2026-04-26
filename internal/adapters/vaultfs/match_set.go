package vaultfs

type stringLike interface {
	~string
}

func normalizeSet[T stringLike](values []T, normalize func(string) string) map[string]struct{} {
	set := make(map[string]struct{})

	for _, value := range values {
		normalized := normalize(string(value))
		if normalized != "" {
			set[normalized] = struct{}{}
		}
	}

	return set
}

func containsAll(searchSet, candidateSet map[string]struct{}) bool {
	for value := range searchSet {
		if _, found := candidateSet[value]; !found {
			return false
		}
	}

	return true
}

func containsAny(searchSet, candidateSet map[string]struct{}) bool {
	for value := range searchSet {
		if _, found := candidateSet[value]; found {
			return true
		}
	}

	return false
}
