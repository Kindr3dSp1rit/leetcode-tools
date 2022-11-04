package graphqlapiservice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

//go:generate mockgen -source=client.go -destination=client_mock_test.go -package=graphql_api_service . httpClient

type (
	httpClient interface {
		Do(req *http.Request) (*http.Response, error)
	}
)

const (
	leetcodeURL        = "https://leetcode.com"
	graphqlAPIEndpoint = leetcodeURL + "/graphql"

	// consult reference/questionData-response.json for fields requested by browser
	problemByTitleSlugQuery = `query questionData($titleSlug: String!) {
	 questionData: question(titleSlug: $titleSlug) {
		questionId
		title
		titleSlug
		exampleTestcases
		codeSnippets {
			langSlug
			code
		}
		content
		isPaidOnly
		canSeeQuestion
		hints
		metaData
	}
}
`
	totalProblemsQuery = `query problemsetQuestionList($categorySlug: String, $filters: QuestionListFilterInput) {
	problemsetQuestionList: questionList(
		categorySlug: $categorySlug
		filters: $filters
	) {
		total: totalNum
	}
}
`
	problemListQuery = `query problemsetQuestionList($categorySlug: String, $filters: QuestionListFilterInput, $limit: Int) {
	problemsetQuestionList: questionList(
		categorySlug: $categorySlug
		filters: $filters
		limit: $limit
	) {
		questions: data {
			title
			titleSlug
			frontendQuestionId: questionFrontendId
		}
	}
}
`
	dailyProblemQuery = `query questionOfToday {
	activeDailyCodingChallengeQuestion {
		question {
			titleSlug
		}
	}
}
`
	variableTitleSlug    = "titleSlug"
	variableCategorySlug = "categorySlug"
	variableFilters      = "filters"
	variableLimit        = "limit"

	csrfTokenCookie        = "csrftoken"
	problemRefererTemplate = leetcodeURL + "/problems/%s/description/"
	problemListReferer     = leetcodeURL + "/problemset/all/"
)

type (
	graphQLRequest struct {
		Query     string         `json:"query"`
		Variables queryVariables `json:"variables"`
	}

	queryVariables map[string]interface{}

	responseDataWrapper struct {
		Data json.RawMessage `json:"data"`
	}

	problemDataResponseWrapper struct {
		Question problemData `json:"questionData"`
	}

	totalProblemsData struct {
		QuestionList struct {
			TotalNum int `json:"total"`
		} `json:"problemsetQuestionList"`
	}

	problemReferenceList struct {
		Questions []problemTitleMap `json:"questions"`
	}

	problemData struct {
		ID        string `json:"questionId"`
		Title     string `json:"title"`
		TitleSlug string `json:"titleSlug"`

		// TODO: parse testcases from problem description
		CodeSnippets []codeSnippet `json:"codeSnippets"`
		Content      string        `json:"content"`

		IsPaidOnly     bool     `json:"isPaidOnly"`
		CanSeeQuestion bool     `json:"canSeeQuestion"`
		Difficulty     string   `json:"difficulty"`
		CategoryTitle  string   `json:"categoryTitle"`
		Hints          []string `json:"hints"`

		MetaDataRaw string   `json:"metaData"` // data on function name and inputs and outputs
		StatsRaw    string   `json:"stats"`
		EnvInfoRaw  string   `json:"envInfo"`
		MetaData    metaData `json:"-"`
		Stats       stats    `json:"-"`
		EnvInfo     envInfo  `json:"-"`
	}

	codeSnippet struct {
		LangSlug string `json:"langSlug"`
		Code     string `json:"code"`
	}

	problemTitleMap struct {
		Title     string `json:"title"`
		TitleSlug string `json:"titleSlug"`
		ID        string `json:"questionFrontendId"`
	}

	dailyChallengeResponse struct {
		Challenge struct {
			Question struct {
				TitleSlug string `json:"titleSlug"`
			} `json:"question"`
		} `json:"activeDailyCodingChallengeQuestion"`
	}

	metaData struct {
		Name   string      `json:"name"`
		Params []parameter `json:"params"`
		Return parameter   `json:"return"`
	}

	parameter struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}

	stats struct {
		TotalAcceptedRaw    int `json:"totalAcceptedRaw"`
		TotalSubmissionsRaw int `json:"totalSubmissionRaw"`
	}

	envInfo map[string][]string // langSlug => [{lang, description}...]
)

func (c *Client) getDailyProblemTitle() (string, error) {
	req, err := c.newRequest(dailyProblemQuery, nil)
	if err != nil {
		return "", fmt.Errorf("init request: %w", err)
	}
	c.addRefererHeader(req, problemListReferer)

	data, err := c.doRequest(req)
	if err != nil {
		return "", fmt.Errorf("query: %w", err)
	}

	parsedResponse := &dailyChallengeResponse{}
	if err = json.Unmarshal(data, parsedResponse); err != nil {
		return "", fmt.Errorf("response unmarshal: %w", err)
	}

	return parsedResponse.Challenge.Question.TitleSlug, nil
}

func (c *Client) getProblemDataByTitleSlug(titleSlug string) (*problemData, error) {
	req, err := c.newRequest(problemByTitleSlugQuery, map[string]interface{}{
		variableTitleSlug: titleSlug,
	})
	if err != nil {
		return nil, fmt.Errorf("init request: %w", err)
	}
	c.addRefererHeader(req, fmt.Sprintf(problemRefererTemplate, titleSlug))

	data, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	fmt.Printf("DATA: %v\n", string(data))

	parsedResponse := &problemDataResponseWrapper{}
	if err = json.Unmarshal(data, parsedResponse); err != nil {
		return nil, fmt.Errorf("response unmarshal: %w", err)
	}

	err = c.parseAdditionalData(&parsedResponse.Question)
	if err != nil {
		return nil, fmt.Errorf("parse fields: %w", err)
	}

	return &parsedResponse.Question, nil
}

func (c *Client) parseAdditionalData(data *problemData) error {
	err := json.Unmarshal([]byte(data.MetaDataRaw), &data.MetaData)
	if err != nil {
		return fmt.Errorf("unmarshal metadata: %w", err)
	}
	err = json.Unmarshal([]byte(data.StatsRaw), &data.Stats)
	if err != nil {
		return fmt.Errorf("unmarshal stats: %w", err)
	}
	err = json.Unmarshal([]byte(data.EnvInfoRaw), &data.EnvInfo)
	if err != nil {
		return fmt.Errorf("unmarshal metadata: %w", err)
	}
	return nil
}

func (c *Client) refreshTitleSlugMaps() error {
	problemCount, err := c.getTotalProblemCount()
	if err != nil {
		return fmt.Errorf("get problem count: %w", err)
	}
	c.mu.RLock()
	if len(c.problemIDMap) == problemCount {
		c.mu.RUnlock()
		return nil
	}

	req, err := c.newRequest(problemListQuery, map[string]interface{}{
		variableCategorySlug: "",
		variableFilters:      struct{}{},
		variableLimit:        problemCount,
	})
	if err != nil {
		return fmt.Errorf("init request: %w", err)
	}
	c.addRefererHeader(req, problemListReferer)

	data, err := c.doRequest(req)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	parsedResponse := &problemReferenceList{}
	if err = json.Unmarshal(data, parsedResponse); err != nil {
		return fmt.Errorf("response unmarshal: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	return nil
}

func (c *Client) getTotalProblemCount() (int, error) {
	req, err := c.newRequest(totalProblemsQuery, map[string]interface{}{
		"categorySlug": "",
		"filters":      struct{}{},
	})
	if err != nil {
		return 0, fmt.Errorf("init request: %w", err)
	}
	c.addRefererHeader(req, problemListReferer)

	data, err := c.doRequest(req)
	if err != nil {
		return 0, fmt.Errorf("query: %w", err)
	}

	parsedResponse := &totalProblemsData{}
	if err = json.Unmarshal(data, parsedResponse); err != nil {
		return 0, fmt.Errorf("response unmarshal: %w", err)
	}

	return parsedResponse.QuestionList.TotalNum, nil
}

func (c *Client) newRequest(query string, variables queryVariables) (*http.Request, error) {
	q := graphQLRequest{
		Query:     query,
		Variables: variables,
	}

	body, err := json.Marshal(q)
	if err != nil {
		return nil, fmt.Errorf("marshal question request: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", graphqlAPIEndpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("initialize request: %w", err)
	}

	err = c.addCSRFHeaders(req)
	if err != nil {
		return nil, fmt.Errorf("add csrf headers: %w", err)
	}
	c.addQueryHeaders(req)

	return req, nil
}

func (c *Client) doRequest(req *http.Request) ([]byte, error) {
	response, err := c.cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read response data: %w", err)
	}
	defer func() {
		cErr := response.Body.Close()
		if cErr != nil {
			fmt.Printf("Error closing API response body: %s\n", err.Error())
		}
	}()

	dataField := &responseDataWrapper{}
	if err = json.Unmarshal(body, dataField); err != nil {
		return nil, fmt.Errorf("response unmarshal: %w", err)
	}

	return dataField.Data, nil
}

func (c *Client) refreshCSRFToken() error {
	c.mu.RLock()
	csrf := c.csrf
	c.mu.RUnlock()

	if csrf != nil && csrf.Expires.After(time.Now().Add(-2*refreshCooldown)) {
		return nil
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", leetcodeURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("request init: %w", err)
	}

	res, err := c.cli.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer func() {
		cErr := res.Body.Close()
		if cErr != nil {
			fmt.Printf("Error closing API response body: %s\n", err.Error())
		}
	}()

	var csrfCookie *http.Cookie
	for _, cookie := range res.Cookies() {
		if cookie.Name == csrfTokenCookie {
			csrfCookie = cookie
			break
		}
	}
	if csrfCookie == nil {
		return fmt.Errorf("no csrftoken cookie found in response")
	}

	c.mu.Lock()
	c.csrf = csrfCookie
	c.mu.Unlock()

	return nil
}

func (c *Client) addCSRFHeaders(req *http.Request) error {
	if c.csrf == nil {
		return fmt.Errorf("csrf token is empty")
	}

	req.AddCookie(c.csrf)
	req.Header.Set("x-csrftoken", c.csrf.Value)

	return nil
}

func (c *Client) addQueryHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
}

func (c *Client) addRefererHeader(req *http.Request, referer string) {
	req.Header.Set("Referer", referer)
}
