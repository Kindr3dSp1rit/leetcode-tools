package graphqlapiservice

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	refreshCooldown = 1 * time.Hour
)

var (
	ErrorProblemNotFound = errors.New("problem not found")
	ErrorSystem          = errors.New("system error")
)

type (
	Client struct {
		cli httpClient

		mu              sync.RWMutex
		csrf            *http.Cookie
		problemIDMap    map[int]string    // id => titleSlug
		problemTitleMap map[string]string // normalized title => titleSlug
		problemCache    cache

		wg    sync.WaitGroup
		close chan struct{}
	}
)

func NewAPIClient() (*Client, error) {
	c := &Client{
		cli: &http.Client{
			Timeout: 10 * time.Second,
		},
		problemCache: newCache(),
	}

	return c, nil
}

func (c *Client) GetProblemByTitle(title string) (Problem, error) {
	if c.problemTitleMap == nil {
		return Problem{}, fmt.Errorf("%w: problem title map is not initialized", ErrorSystem)
	}
	c.mu.RLock()
	titleSlug, ok := c.problemTitleMap[title]
	c.mu.RUnlock()
	if !ok {
		return Problem{}, ErrorProblemNotFound
	}

	return c.GetProblemByTitleSlug(titleSlug)
}

func (c *Client) GetProblemByID(id int) (Problem, error) {
	if c.problemIDMap == nil {
		return Problem{}, fmt.Errorf("%w: problem title map is not initialized", ErrorSystem)
	}
	c.mu.RLock()
	titleSlug, ok := c.problemIDMap[id]
	c.mu.RUnlock()
	if !ok {
		return Problem{}, ErrorProblemNotFound
	}

	return c.GetProblemByTitleSlug(titleSlug)
}

func (c *Client) GetProblemByTitleSlug(titleSlug string) (Problem, error) {
	if p, cacheHit := c.problemCache.get(titleSlug); cacheHit {
		return p, nil
	}

	data, err := c.getProblemDataByTitleSlug(titleSlug)
	if err != nil {
		return Problem{}, fmt.Errorf("%w: get problem data from API", ErrorSystem)
	}

	problem, err := externalProblemFromProblemData(data)
	if err != nil {
		return Problem{}, fmt.Errorf("%w: ", ErrorSystem)
	}
	c.problemCache.add(&problem)

	return problem, nil
}

func (c *Client) GetDailyProblem() (Problem, error) {
	titleSlug, err := c.getDailyProblemTitle()
	if err != nil {
		return Problem{}, fmt.Errorf("%w: get daily problem title", ErrorSystem)
	}

	if p, cacheHit := c.problemCache.get(titleSlug); cacheHit {
		return p, nil
	}

	data, err := c.getProblemDataByTitleSlug(titleSlug)
	if err != nil {
		return Problem{}, fmt.Errorf("%w:: get problem data from API", ErrorSystem)
	}

	problem, err := externalProblemFromProblemData(data)
	if err != nil {
		return Problem{}, fmt.Errorf("%w: ", ErrorSystem)
	}
	c.problemCache.add(&problem)

	return problem, nil
}

func (c *Client) Run() {
	log.Println("Running LeetCode GraphQL API Service")

	c.problemCache.run()

	c.wg.Add(1)
	go func() {
		ticker := time.NewTicker(refreshCooldown)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := c.refreshCSRFToken()
				if err != nil {
					log.Println("Error refreshing csrf token: %w", err)
				}

				err = c.refreshTitleSlugMaps()
				if err != nil {
					log.Println("Error refreshing problem title maps: %w", err)
				}

			case <-c.close:
				c.problemIDMap = nil
				c.problemTitleMap = nil
				return
			}
		}
	}()
}

func (c *Client) Stop() {
	log.Println("Stopping LeetCode GraphQL API Service")
	c.problemCache.stop()
	c.close <- struct{}{}
	c.wg.Wait()
}
