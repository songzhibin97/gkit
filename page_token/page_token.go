package page_token

// https://google.aip.dev/158

type PageToken interface {
	TokenGenerator
	ProcessPageTokens
}

type TokenGenerator interface {
	ForIndex(int) string
	GetIndex(string) (int, error)
}

type ProcessPageTokens interface {
	// ProcessPageTokens
	// numElements: total number of elements
	// pageSize: number of elements per page
	// pageToken: page token

	ProcessPageTokens(numElements int, pageSize int, pageToken string) (start, end int, nextToken string, err error)
}
