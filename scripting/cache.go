package scripting

import (
	"errors"
	"fmt"
	"sync"

	"github.com/tychoish/jasper"
	"github.com/tychoish/jasper/options"
)

// NewCache constructs a threadsafe HarnessCache instance.
func NewCache() HarnessCache { return &cacheImpl{cache: make(map[string]Harness)} }

type cacheImpl struct {
	cache map[string]Harness
	mutex sync.RWMutex
}

func (c *cacheImpl) Create(jpm jasper.Manager, opts options.ScriptingHarness) (Harness, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	h, err := NewHarness(jpm, opts)
	if err != nil {
		return nil, fmt.Errorf("problem constructing harness: %w", err)
	}

	c.cache[h.ID()] = h

	return h, nil
}

func (c *cacheImpl) Add(id string, h Harness) error {
	if h == nil {
		return errors.New("cannot cache nil harness")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, ok := c.cache[id]; ok {
		return fmt.Errorf("harness '%s' exists, cannot cache", id)
	}

	c.cache[id] = h

	return nil
}

func (c *cacheImpl) Get(id string) (Harness, error) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if h, ok := c.cache[id]; ok {
		return h, nil
	}

	return nil, fmt.Errorf("could not find manager '%s'", id)
}

func (c *cacheImpl) Check(id string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if _, ok := c.cache[id]; ok {
		return true
	}

	return false
}
