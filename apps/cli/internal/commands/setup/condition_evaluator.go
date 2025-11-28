package setup

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// ConditionEvaluator evaluates conditional expressions
type ConditionEvaluator struct {
	state    *types.SetupState
	platform platform.DetectionResult
}

// NewConditionEvaluator creates a new condition evaluator
func NewConditionEvaluator(state *types.SetupState, platform platform.DetectionResult) *ConditionEvaluator {
	return &ConditionEvaluator{
		state:    state,
		platform: platform,
	}
}

// Evaluate evaluates a condition and returns true if it matches
func (ce *ConditionEvaluator) Evaluate(condition *types.Condition) (bool, error) {
	// Handle logical operators first
	if condition.And != nil {
		return ce.evaluateAnd(condition.And)
	}
	if condition.Or != nil {
		return ce.evaluateOr(condition.Or)
	}
	if condition.Not != nil {
		result, err := ce.Evaluate(condition.Not)
		return !result, err
	}

	// Handle system conditions
	if condition.System != nil {
		return ce.evaluateSystemCondition(condition.System), nil
	}

	// Handle variable conditions
	if condition.Variable != "" {
		return ce.evaluateVariableCondition(condition)
	}

	// No valid condition specified
	return true, nil
}

// evaluateAnd evaluates AND conditions
func (ce *ConditionEvaluator) evaluateAnd(conditions []*types.Condition) (bool, error) {
	for _, cond := range conditions {
		result, err := ce.Evaluate(cond)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

// evaluateOr evaluates OR conditions
func (ce *ConditionEvaluator) evaluateOr(conditions []*types.Condition) (bool, error) {
	for _, cond := range conditions {
		result, err := ce.Evaluate(cond)
		if err != nil {
			return false, err
		}
		if result {
			return true, nil
		}
	}
	return false, nil
}

// evaluateSystemCondition evaluates system-level conditions
func (ce *ConditionEvaluator) evaluateSystemCondition(sys *types.SystemCondition) bool {
	// Check OS
	if sys.OS != "" && !ce.matchString(ce.platform.OS, sys.OS) {
		return false
	}

	// Check distribution
	if sys.Distribution != "" && !ce.matchString(ce.platform.Distribution, sys.Distribution) {
		return false
	}

	// Check desktop environment
	if sys.Desktop != "" && !ce.matchString(ce.platform.DesktopEnv, sys.Desktop) {
		return false
	}

	// Check architecture
	if sys.Architecture != "" && !ce.matchString(ce.platform.Architecture, sys.Architecture) {
		return false
	}

	// Check has desktop
	if sys.HasDesktop != nil {
		hasDesktop := ce.platform.DesktopEnv != "none" &&
			ce.platform.DesktopEnv != "unknown" &&
			ce.platform.DesktopEnv != ""
		if hasDesktop != *sys.HasDesktop {
			return false
		}
	}

	return true
}

// evaluateVariableCondition evaluates variable-based conditions
func (ce *ConditionEvaluator) evaluateVariableCondition(condition *types.Condition) (bool, error) {
	// Get variable value from state
	var varValue interface{}
	var exists bool

	// Check in answers first
	varValue, exists = ce.state.Answers[condition.Variable]
	if !exists {
		// Check in system info
		varValue, exists = ce.state.SystemInfo[condition.Variable]
	}

	// Evaluate based on operator
	switch condition.Operator {
	case types.OperatorExists:
		return exists && varValue != nil && varValue != "", nil

	case types.OperatorNotExists:
		return !exists || varValue == nil || varValue == "", nil

	case types.OperatorEquals:
		if !exists {
			return false, nil
		}
		return ce.compareValues(varValue, condition.Value, true), nil

	case types.OperatorNotEquals:
		if !exists {
			return true, nil
		}
		return ce.compareValues(varValue, condition.Value, false), nil

	case types.OperatorContains:
		if !exists {
			return false, nil
		}
		return ce.containsValue(varValue, condition.Value), nil

	case types.OperatorNotContains:
		if !exists {
			return true, nil
		}
		return !ce.containsValue(varValue, condition.Value), nil

	case types.OperatorMatches:
		if !exists {
			return false, nil
		}
		return ce.matchesPattern(varValue, condition.Value)

	case types.OperatorNotMatches:
		if !exists {
			return true, nil
		}
		matches, err := ce.matchesPattern(varValue, condition.Value)
		return !matches, err

	case types.OperatorGreaterThan, types.OperatorLessThan:
		if !exists {
			return false, nil
		}
		return ce.compareNumeric(varValue, condition.Value, condition.Operator)

	default:
		return false, fmt.Errorf("unknown operator: %s", condition.Operator)
	}
}

// compareValues compares two values for equality
func (ce *ConditionEvaluator) compareValues(a, b interface{}, expectEqual bool) bool {
	// Convert both to strings for comparison
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	equal := aStr == bStr
	if expectEqual {
		return equal
	}
	return !equal
}

// containsValue checks if a value contains another value
func (ce *ConditionEvaluator) containsValue(haystack, needle interface{}) bool {
	// Handle slice/array types
	if slice, ok := haystack.([]interface{}); ok {
		needleStr := fmt.Sprintf("%v", needle)
		for _, item := range slice {
			if fmt.Sprintf("%v", item) == needleStr {
				return true
			}
		}
		return false
	}

	// Handle string types
	haystackStr := fmt.Sprintf("%v", haystack)
	needleStr := fmt.Sprintf("%v", needle)
	return strings.Contains(haystackStr, needleStr)
}

// matchesPattern checks if a value matches a regex pattern
func (ce *ConditionEvaluator) matchesPattern(value, pattern interface{}) (bool, error) {
	valueStr := fmt.Sprintf("%v", value)
	patternStr := fmt.Sprintf("%v", pattern)

	matched, err := regexp.MatchString(patternStr, valueStr)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern: %w", err)
	}
	return matched, nil
}

// compareNumeric compares two values numerically
func (ce *ConditionEvaluator) compareNumeric(a, b interface{}, operator types.ConditionOperator) (bool, error) {
	// Try to convert to float for comparison
	aFloat, aOk := ce.toFloat(a)
	bFloat, bOk := ce.toFloat(b)

	if !aOk || !bOk {
		return false, fmt.Errorf("cannot compare non-numeric values")
	}

	switch operator {
	case types.OperatorGreaterThan:
		return aFloat > bFloat, nil
	case types.OperatorLessThan:
		return aFloat < bFloat, nil
	default:
		return false, fmt.Errorf("invalid numeric operator: %s", operator)
	}
}

// toFloat attempts to convert a value to float64
func (ce *ConditionEvaluator) toFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	default:
		// Try to parse string
		if str, ok := value.(string); ok {
			var f float64
			if _, err := fmt.Sscanf(str, "%f", &f); err == nil {
				return f, true
			}
		}
		return 0, false
	}
}

// matchString checks if a string matches a pattern (supports wildcards)
func (ce *ConditionEvaluator) matchString(value, pattern string) bool {
	// Exact match
	if value == pattern {
		return true
	}

	// Wildcard match
	if strings.Contains(pattern, "*") {
		// Convert glob pattern to regex
		regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*") + "$"
		matched, err := regexp.MatchString(regexPattern, value)
		return err == nil && matched
	}

	return false
}
