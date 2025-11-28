package setup_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands/setup"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("ConditionEvaluator", func() {
	var (
		evaluator        *setup.ConditionEvaluator
		state            *types.SetupState
		detectedPlatform platform.DetectionResult
	)

	BeforeEach(func() {
		state = &types.SetupState{
			Answers: map[string]interface{}{
				"username":     "john",
				"email":        "john@example.com",
				"age":          25,
				"is_admin":     true,
				"languages":    []interface{}{"go", "python", "rust"},
				"empty_string": "",
			},
			SystemInfo: map[string]interface{}{
				"os":           "linux",
				"distribution": "debian",
				"desktop":      "gnome",
				"has_desktop":  true,
			},
		}

		detectedPlatform = platform.DetectionResult{
			OS:           "linux",
			Distribution: "debian",
			DesktopEnv:   "gnome",
			Architecture: "amd64",
		}

		evaluator = setup.NewConditionEvaluator(state, detectedPlatform)
	})

	Describe("Evaluate", func() {
		Context("with Equals operator", func() {
			It("should return true when values match", func() {
				condition := &types.Condition{
					Operator: types.OperatorEquals,
					Variable: "username",
					Value:    "john",
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should return false when values don't match", func() {
				condition := &types.Condition{
					Operator: types.OperatorEquals,
					Variable: "username",
					Value:    "jane",
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		Context("with NotEquals operator", func() {
			It("should return true when values don't match", func() {
				condition := &types.Condition{
					Operator: types.OperatorNotEquals,
					Variable: "username",
					Value:    "jane",
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})
		})

		Context("with Contains operator", func() {
			It("should return true when array contains value", func() {
				condition := &types.Condition{
					Operator: types.OperatorContains,
					Variable: "languages",
					Value:    "python",
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should return true when string contains substring", func() {
				condition := &types.Condition{
					Operator: types.OperatorContains,
					Variable: "email",
					Value:    "example",
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})
		})

		Context("with Exists operator", func() {
			It("should return true for existing variable", func() {
				condition := &types.Condition{
					Operator: types.OperatorExists,
					Variable: "username",
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should return false for empty variable", func() {
				condition := &types.Condition{
					Operator: types.OperatorExists,
					Variable: "empty_string",
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		Context("with System condition", func() {
			It("should match OS", func() {
				condition := &types.Condition{
					System: &types.SystemCondition{
						OS: "linux",
					},
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should match distribution", func() {
				condition := &types.Condition{
					System: &types.SystemCondition{
						Distribution: "debian",
					},
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should match desktop", func() {
				condition := &types.Condition{
					System: &types.SystemCondition{
						Desktop: "gnome",
					},
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should not match wrong OS", func() {
				condition := &types.Condition{
					System: &types.SystemCondition{
						OS: "darwin",
					},
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		Context("with And condition", func() {
			It("should return true when all conditions are true", func() {
				condition := &types.Condition{
					And: []*types.Condition{
						{
							Operator: types.OperatorEquals,
							Variable: "username",
							Value:    "john",
						},
						{
							Operator: types.OperatorEquals,
							Variable: "is_admin",
							Value:    true,
						},
					},
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})

			It("should return false when any condition is false", func() {
				condition := &types.Condition{
					And: []*types.Condition{
						{
							Operator: types.OperatorEquals,
							Variable: "username",
							Value:    "john",
						},
						{
							Operator: types.OperatorEquals,
							Variable: "username",
							Value:    "jane",
						},
					},
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})

		Context("with Or condition", func() {
			It("should return true when any condition is true", func() {
				condition := &types.Condition{
					Or: []*types.Condition{
						{
							Operator: types.OperatorEquals,
							Variable: "username",
							Value:    "jane",
						},
						{
							Operator: types.OperatorEquals,
							Variable: "username",
							Value:    "john",
						},
					},
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeTrue())
			})
		})

		Context("with Not condition", func() {
			It("should invert result", func() {
				condition := &types.Condition{
					Not: &types.Condition{
						Operator: types.OperatorEquals,
						Variable: "username",
						Value:    "john",
					},
				}

				result, err := evaluator.Evaluate(condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(BeFalse())
			})
		})
	})
})
