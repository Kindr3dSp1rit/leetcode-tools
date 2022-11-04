package graphqlapiservice

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnit_ExportProblemFromProblemData(t *testing.T) {
	testCases := map[string]struct {
		data     problemData
		expected Problem
		err      error
	}{
		"normal conversion": {
			data: problemData{
				ID:             "1",
				Title:          "Test Problem",
				TitleSlug:      "test-problem",
				CodeSnippets:   []codeSnippet{{LangSlug: "golang", Code: "<golang code>"}},
				Content:        "123",
				IsPaidOnly:     false,
				CanSeeQuestion: false,
				Difficulty:     "easy",
				CategoryTitle:  "Algorithms",
				Hints:          []string{"hint #1", "hint #2"},
				MetaDataRaw:    "", // these fields shouldn't be used
				StatsRaw:       "",
				EnvInfoRaw:     "",
				MetaData: metaData{
					Name:   "testProblem",
					Params: []parameter{{Name: "input", Type: "integer[]"}},
					Return: parameter{Type: "integer[]"},
				},
				Stats:   stats{TotalAcceptedRaw: 10, TotalSubmissionsRaw: 20},
				EnvInfo: envInfo{"golang": {"Go", "<golang env info>"}},
			},
			expected: Problem{
				ID:        1,
				Title:     "Test Problem",
				TitleSlug: "test-problem",
				MetaData: MetaData{
					FunctionName:    "testProblem",
					InputParameters: []Parameter{{Name: "input", Type: "integer[]"}},
					ReturnParameter: Parameter{Type: "integer[]"},
				},
				CodeSnippets:   map[string]string{"golang": "<golang code>"},
				Stats:          Stats{TotalAccepted: 10, TotalSubmissions: 20},
				EnvInfo:        map[string]string{"golang": "<golang env info>"},
				IsPaidOnly:     false,
				CanSeeQuestion: false,
				Difficulty:     "easy",
				CategoryTitle:  "Algorithms",
				Hints:          []string{"hint #1", "hint #2"},
			},
			err: nil,
		},
		"id error": {
			data: problemData{
				ID: "asdfgh",
			},
			expected: Problem{},
			err:      fmt.Errorf("invalid id string: asdfgh"),
		},
		"env info error": {
			data: problemData{
				ID:      "1",
				EnvInfo: map[string][]string{"golang": {}},
			},
			expected: Problem{},
			err:      fmt.Errorf("invalid env info for golang: []"),
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			p, err := externalProblemFromProblemData(&test.data)
			assert.Equal(t, test.expected, p)
			assert.Equal(t, test.err, err)
		})
	}
}
