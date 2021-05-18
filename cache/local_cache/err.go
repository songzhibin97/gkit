package local_cache

import "errors"

var (
	CacheExist       = errors.New("local_cache: cache exist")
	CacheNoExist     = errors.New("local_cache: cache no exist")
	CacheExpire      = errors.New("local_cache: cache expire")
	CacheIncrTypeErr = errors.New("local_cache: cache incr type err")
	CacheGobErr      = errors.New("local_cache: cache save gob err")
)

func Exist(e error) bool {
	return errors.Is(e, CacheExist)
}

func NoExist(e error) bool {
	return errors.Is(e, CacheNoExist)
}

func Expire(e error) bool {
	return errors.Is(e, CacheExpire)
}

func IncrTypeErr(e error) bool {
	return errors.Is(e, CacheIncrTypeErr)
}
