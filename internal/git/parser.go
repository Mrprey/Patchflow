package git

import (
	"sort"
	"strings"
)

func ParseRemotes(output string) []Remote {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	seen := make(map[string]Remote)
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := fields[0]
		url := fields[1]
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = Remote{Name: name, URL: url}
	}
	remotes := make([]Remote, 0, len(seen))
	for _, remote := range seen {
		remotes = append(remotes, remote)
	}
	sort.Slice(remotes, func(i, j int) bool { return remotes[i].Name < remotes[j].Name })
	return remotes
}

func ParseLog(output string) []Commit {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	commits := make([]Commit, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		commits = append(commits, Commit{
			SHA:     fields[0],
			ShortSHA: fields[1],
			Author:  fields[2],
			Date:    fields[3],
			Title:   fields[4],
		})
	}
	return commits
}
