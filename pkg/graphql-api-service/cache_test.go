package graphqlapiservice

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUnit_Cache(t *testing.T) {
	c := newCache()

	testProblem := Problem{
		ID:        1,
		Title:     "Test Problem",
		TitleSlug: "test-problem",
	}
	c.add(&testProblem)
	p, ok := c.get("test-problem")
	assert.True(t, ok)
	assert.Equal(t, testProblem, p)

	p, ok = c.get("unknown-problem")
	assert.False(t, ok)
	assert.Equal(t, Problem{}, p)

	c.problems["123"] = entry{
		problem: Problem{ID: 2},
		expires: time.Now(),
	}
	_, ok = c.get("123")
	assert.True(t, ok)

	c.cleanup()
	_, ok = c.get("123")
	assert.False(t, ok)
}
