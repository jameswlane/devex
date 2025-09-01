package common

import "fmt"

// ValidateNonEmpty checks that a string is not empty.
func ValidateNonEmpty(value, name string) error {
	if value == "" {
		return fmt.Errorf("%s cannot be empty", name)
	}
	return nil
}

// ValidateListNotEmpty checks that a list is not empty.
func ValidateListNotEmpty(list []string, name string) error {
	if len(list) == 0 {
		return fmt.Errorf("%s cannot be empty", name)
	}
	return nil
}

// ValidateAppConfig ensures an AppConfig has all required fields.
func ValidateAppConfig(name, method string) error {
	if name == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	if method == "" {
		return fmt.Errorf("install method cannot be empty for app %s", name)
	}
	return nil
}
