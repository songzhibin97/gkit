package page_token

import (
	"encoding/base64"
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
	v := aes.Encrypt(fmt.Sprintf("%s%s:%d", t.resourceIdentification, time.Now().Format(layout), i), t.salt)
	return base64.StdEncoding.EncodeToString(
		[]byte(v))
}

func (t *token) GetIndex(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	bs, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return -1, ErrInvalidToken
	}
	decrypted := aes.Decrypt(string(bs), t.salt)
	if decrypted == "" {
		return -1, ErrInvalidToken
	}
	parseToken := strings.Split(strings.TrimPrefix(string(decrypted), t.resourceIdentification), ":")
	if len(parseToken) != 2 {
		return -1, ErrInvalidToken
	}
	if t.timeLimitation != 0 {
		generateTime, err := time.Parse(layout, parseToken[0])
		if err != nil {
			return -1, ErrInvalidToken
		}
		if generateTime.Add(t.timeLimitation).After(time.Now()) {
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
