package shared

import "fmt"

// GenerateTasks generates tasks based on shared selections.
func GenerateTasks(shared map[string][]string) []string {
	tasks := []string{}

	if languages, ok := shared["Programming Languages"]; ok {
		for _, lang := range languages {
			tasks = append(tasks, fmt.Sprintf("Installing %s", lang))
		}
	}
	if databases, ok := shared["Databases"]; ok {
		for _, db := range databases {
			tasks = append(tasks, fmt.Sprintf("Installing %s", db))
		}
	}
	if themes, ok := shared["Themes"]; ok && len(themes) > 0 {
		tasks = append(tasks, fmt.Sprintf("Applying theme: %s", themes[0]))
	}

	return tasks
}
