package pool

import (
	"errors"
)

const (
	// DefaultPoolSize 是Pool默认的capacity
	DefaultPoolSize = 10000

	// DefaultCleanInterval 是pool默认的情况清理时间
	DefaultCleanInterval = 5
)

// 默认的Pool
var defaultPool, _ = NewPool(DefaultPoolSize)

func Submit(task *job) error {
	return defaultPool.Submit(task)
}

func Running() int {
	return defaultPool.Running()
}

func Cap() int {
	return defaultPool.Cap()
}

func Idle() int {
	return defaultPool.Idle()
}

func Close() {
	defaultPool.Close()
}

var (
	ErrInvalidPoolSize   = errors.New("invalid size for pool")
	ErrInvalidPoolExpiry = errors.New("invalid expiry for pool")
	ErrPoolClosed        = errors.New("this pool has been closed")
	ErrFunction          = errors.New("function type is invalid")
	ErrFunctionArgs      = errors.New("function args is invalid")
)
