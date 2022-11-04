package graphqlapiservice

import (
	"sync"
	"time"
)

const (
	problemExpiration = 1 * time.Hour
	cacheTick         = 1 * time.Minute
)

type (
	cache struct {
		sync.RWMutex
		close    chan struct{}
		problems map[string]entry
	}

	entry struct {
		problem Problem
		expires time.Time
	}
)

func newCache() cache {
	return cache{
		problems: make(map[string]entry),
	}
}

func (c *cache) run() {
	go func() {
		ticker := time.NewTicker(cacheTick)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.cleanup()
			case <-c.close:
				return
			}
		}
	}()
}

func (c *cache) cleanup() {
	c.Lock()
	defer c.Unlock()
	for titleSlug := range c.problems {
		if c.problems[titleSlug].expires.Before(time.Now()) {
			delete(c.problems, titleSlug)
		}
	}
}

func (c *cache) stop() {
	c.close <- struct{}{}
}

func (c *cache) add(p *Problem) {
	c.Lock()
	defer c.Unlock()

	c.problems[p.TitleSlug] = entry{
		problem: *p,
		expires: time.Now().Add(problemExpiration),
	}
}

func (c *cache) get(titleSlug string) (Problem, bool) {
	c.RLock()
	defer c.RUnlock()

	e, ok := c.problems[titleSlug]
	return e.problem, ok
}
