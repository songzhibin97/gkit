package page_token

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/songzhibin97/gkit/options"

	"github.com/songzhibin97/gkit/encrypt/aes"
)

var (
	ErrInvalidToken         = errors.New("the field `page_token` is invalid")
	ErrOverdueToken         = errors.New("the field `page_token` is overdue")
	ErrOverMaxPageSizeToken = errors.New("the field `page_token` is over max page size")
	ErrInvalidPageSize      = errors.New("the page size provided must not be negative")

	// ErrDefaultSalt is returned by NewTokenGenerateE when no SetSalt option
	// has been supplied. The default salt "gkit" is hard-coded in this package
	// and any attacker who reads the source can forge tokens; production
	// callers must always supply their own salt.
	ErrDefaultSalt = errors.New("page_token: SetSalt option is required (default salt is publicly known)")
)

const (
	defaultMaxIndex    = 0
	defaultMaxElements = 0
	defaultSalt        = "gkit"
	layout             = "2006-01-02 15-04-05"
)

type token struct {
	salt                   string        // Special identification
	resourceIdentification string        // Resource identification
	timeLimitation         time.Duration // Time limitation
	maxIndex               int           // Maximum index
	maxElements            int           // Maximum number of elements
}

func (t *token) ForIndex(i int) string {
	v, err := aes.Encrypt(fmt.Sprintf("%s%s:%d", t.resourceIdentification, time.Now().Format(layout), i), t.salt)
	if err != nil {
		return ""
	}
	return v
}

func (t *token) GetIndex(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	decrypted, err := aes.Decrypt(s, t.salt)
	if err != nil {
		return -1, ErrInvalidToken
	}
	if decrypted == "" {
		return -1, ErrInvalidToken
	}
	if !strings.HasPrefix(decrypted, t.resourceIdentification) {
		// A token whose plaintext does not start with the expected resource
		// prefix is from a different resource (or forged). TrimPrefix silently
		// returns the input unchanged in that case, so without this check a
		// cross-resource token could be replayed.
		return -1, ErrInvalidToken
	}
	parseToken := strings.Split(strings.TrimPrefix(decrypted, t.resourceIdentification), ":")
	if len(parseToken) != 2 {
		return -1, ErrInvalidToken
	}
	if t.timeLimitation != 0 {
		generateTime, err := time.ParseInLocation(layout, parseToken[0], time.Local)
		if err != nil {
			return -1, ErrInvalidToken
		}
		if generateTime.Add(t.timeLimitation).Before(time.Now()) {
			return -1, ErrOverdueToken
		}
	}
	i, err := strconv.Atoi(parseToken[1])
	if err != nil {
		return -1, ErrInvalidToken
	}
	if t.maxIndex != defaultMaxIndex && i > t.maxIndex {
		return -1, ErrOverMaxPageSizeToken
	}
	return i, nil
}

func (t *token) ProcessPageTokens(numElements int, pageSize int, pageToken string) (start, end int, nextToken string, err error) {
	if pageSize < 0 {
		return 0, 0, "", ErrInvalidPageSize
	}

	if t.maxElements != defaultMaxElements && numElements > t.maxElements {
		numElements = t.maxElements
	}

	if pageToken != "" {
		index, err := t.GetIndex(pageToken)
		if err != nil {
			return 0, 0, "", err
		}

		token64 := index
		if token64 < 0 || token64 >= numElements {
			return 0, 0, "", ErrInvalidToken
		}
		start = token64
	}

	if pageSize == 0 {
		pageSize = numElements
	}
	end = min(start+pageSize, numElements)

	if end < numElements {
		nextToken = t.ForIndex(int(end))
	}

	return start, end, nextToken, nil
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

// NewTokenGenerate constructs a PageToken using the package's hard-coded
// default salt unless overridden by SetSalt.
//
// Deprecated: tokens produced with the default salt are forgeable by anyone
// who reads the gkit source. Use NewTokenGenerateE and supply SetSalt.
func NewTokenGenerate(resourceIdentification string, options ...options.Option) PageToken {
	t := &token{
		maxIndex:               defaultMaxIndex,
		maxElements:            defaultMaxElements,
		timeLimitation:         0,
		salt:                   aes.PadKey(defaultSalt),
		resourceIdentification: resourceIdentification,
	}
	for _, option := range options {
		option(t)
	}
	return t
}

// NewTokenGenerateE constructs a PageToken and requires the caller to provide
// a non-default salt via SetSalt. It returns ErrDefaultSalt otherwise.
func NewTokenGenerateE(resourceIdentification string, opts ...options.Option) (PageToken, error) {
	defaultPadded := aes.PadKey(defaultSalt)
	t := &token{
		maxIndex:               defaultMaxIndex,
		maxElements:            defaultMaxElements,
		timeLimitation:         0,
		salt:                   defaultPadded,
		resourceIdentification: resourceIdentification,
	}
	for _, option := range opts {
		option(t)
	}
	if t.salt == "" || t.salt == defaultPadded {
		return nil, ErrDefaultSalt
	}
	return t, nil
}
