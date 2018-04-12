package main

import "fmt"

type Pipeline struct {
	Name     string `json:"name"`
	Paused   bool   `json:"paused"`
	TeamName string `json:"team_name"`
	Running  bool
	Statuses map[string]int
}

type Job struct {
	NextBuild struct {
		Status string `json:"status"`
	} `json:"next_build"`
	Build struct {
		Status string `json:"status"`
	} `json:"finished_build"`
}

func GetData() []Pipeline {
	pipelines := make([]Pipeline, 0, 0)
	if err := GetJSON("https://"+hostName+"/api/v1/pipelines", &pipelines); err != nil {
		panic(err)
	}
	for idx, pipeline := range pipelines {
		url := fmt.Sprintf(
			"https://%s/api/v1/teams/%s/pipelines/%s/jobs",
			hostName,
			pipeline.TeamName,
			pipeline.Name,
		)
		jobs := make([]Job, 0, 0)
		if err := GetJSON(url, &jobs); err != nil {
			panic(err)
		}
		pipelines[idx].Statuses = map[string]int{}
		for _, job := range jobs {
			pipelines[idx].Statuses[job.Build.Status]++
			if job.NextBuild.Status != "" {
				pipelines[idx].Running = true
			}
		}
	}
	return pipelines
}
