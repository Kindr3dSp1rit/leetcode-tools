package graphqlapiservice

import (
	"fmt"
	"strconv"
)

// TODO: add lang list
const (
	LangSlugGolang = "golang"
)

type (
	LeetCodeAPIClient interface {
		GetProblemDataByTitle(title string) (Problem, error)
		GetProblemDataByID(id int) (Problem, error)
		GetDailyProblem() (Problem, error)
	}

	Problem struct {
		ID        int // frontend id
		Title     string
		TitleSlug string

		MetaData     MetaData
		CodeSnippets map[string]string // langSlug => code
		Stats        Stats
		EnvInfo      map[string]string // langSlug => envInfo

		IsPaidOnly     bool
		CanSeeQuestion bool
		Difficulty     string
		CategoryTitle  string
		Hints          []string
	}

	MetaData struct {
		FunctionName    string
		InputParameters []Parameter
		ReturnParameter Parameter
	}

	Parameter struct {
		Name string
		Type string
	}

	Stats struct {
		TotalAccepted    int
		TotalSubmissions int
	}
)

func externalProblemFromProblemData(data *problemData) (Problem, error) {
	p := Problem{
		Title:     data.Title,
		TitleSlug: data.TitleSlug,
		Stats: Stats{
			TotalAccepted:    data.Stats.TotalAcceptedRaw,
			TotalSubmissions: data.Stats.TotalSubmissionsRaw,
		},
		IsPaidOnly:     data.IsPaidOnly,
		CanSeeQuestion: data.CanSeeQuestion,
		Difficulty:     data.Difficulty,
		CategoryTitle:  data.CategoryTitle,
		Hints:          data.Hints,
	}

	id, err := strconv.Atoi(data.ID)
	if err != nil {
		return Problem{}, fmt.Errorf("invalid id string: %s", data.ID)
	}
	p.ID = id

	p.CodeSnippets = make(map[string]string, len(data.CodeSnippets))
	for _, c := range data.CodeSnippets {
		p.CodeSnippets[c.LangSlug] = c.Code
	}

	p.MetaData = MetaData{
		FunctionName: data.MetaData.Name,
		ReturnParameter: Parameter{
			Name: data.MetaData.Return.Name,
			Type: data.MetaData.Return.Type,
		},
	}

	params := make([]Parameter, 0, len(p.MetaData.InputParameters))
	for _, i := range data.MetaData.Params {
		params = append(params, Parameter(i))
	}
	p.MetaData.InputParameters = params

	envinfo := make(map[string]string, len(data.EnvInfo))
	for langSlug, env := range data.EnvInfo {
		if len(env) < 2 {
			return Problem{}, fmt.Errorf("invalid env info for %s: %s", langSlug, env)
		}
		envinfo[langSlug] = env[1]
	}
	p.EnvInfo = envinfo

	return p, nil
}
