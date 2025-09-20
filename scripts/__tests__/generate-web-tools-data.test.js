/**
 * @jest-environment node
 */
const { exec } = require("child_process");
const fs = require("fs");
const path = require("path");

// Mock dependencies
jest.mock("fs");
jest.mock("path");
jest.mock("js-yaml");
jest.mock("glob");

const yaml = require("js-yaml");
const { glob } = require("glob");

// Import the functions we want to test
// Note: Since the script uses module.exports at the end, we need to require it
const scriptPath = path.join(__dirname, "../generate-web-tools-data.js");

// Mock console methods to reduce noise
const consoleSpy = {
	log: jest.spyOn(console, "log").mockImplementation(() => {}),
	warn: jest.spyOn(console, "warn").mockImplementation(() => {}),
	error: jest.spyOn(console, "error").mockImplementation(() => {}),
};

afterEach(() => {
	Object.values(consoleSpy).forEach((spy) => spy.mockClear());
	jest.clearAllMocks();
});

afterAll(() => {
	Object.values(consoleSpy).forEach((spy) => spy.mockRestore());
});

describe("YAML Validation Functions", () => {
	// We need to test the validation functions by executing them
	let validateApplicationData;
	let validatePluginData;
	let sanitizeCommand;

	beforeAll(async () => {
		// Load the script and extract functions for testing
		const scriptContent = fs.readFileSync(scriptPath, "utf8");

		// Create a test version that exports the functions
		const testScript = scriptContent.replace(
			"module.exports = { generateToolsData };",
			`
      module.exports = { 
        generateToolsData,
        validateApplicationData,
        validatePluginData,
        sanitizeCommand
      };
      `,
		);

		// Write test script
		const testScriptPath = path.join(__dirname, "temp-test-script.js");
		fs.writeFileSync(testScriptPath, testScript);

		// Import functions
		const testModule = require(testScriptPath);
		validateApplicationData = testModule.validateApplicationData;
		validatePluginData = testModule.validatePluginData;
		sanitizeCommand = testModule.sanitizeCommand;

		// Cleanup
		fs.unlinkSync(testScriptPath);
	});

	describe("sanitizeCommand", () => {
		it("should remove dangerous shell metacharacters", () => {
			const dangerousCmd = "apt install test `echo dangerous` && rm -rf /";
			const sanitized = sanitizeCommand(dangerousCmd);

			expect(sanitized).not.toContain("`");
			expect(sanitized).not.toContain("&&");
			expect(sanitized).not.toContain("|");
			expect(sanitized).toBe("apt install test echo dangerous  rm -rf /");
		});

		it("should remove escape sequences", () => {
			const cmdWithEscapes = "echo \\n\\t\\r test";
			const sanitized = sanitizeCommand(cmdWithEscapes);

			expect(sanitized).toBe("echo  test");
		});

		it("should normalize whitespace", () => {
			const cmdWithExtraSpaces = "apt   install    test";
			const sanitized = sanitizeCommand(cmdWithExtraSpaces);

			expect(sanitized).toBe("apt install test");
		});

		it("should handle empty or invalid input", () => {
			expect(sanitizeCommand("")).toBe("");
			expect(sanitizeCommand(null)).toBe("");
			expect(sanitizeCommand(undefined)).toBe("");
			expect(sanitizeCommand(123)).toBe("");
		});

		it("should preserve safe commands", () => {
			const safeCmd = "apt install nodejs npm git";
			const sanitized = sanitizeCommand(safeCmd);

			expect(sanitized).toBe("apt install nodejs npm git");
		});
	});

	describe("validateApplicationData", () => {
		it("should pass validation for valid application data", () => {
			const validData = {
				name: "Test Application",
				description: "A test application for validation",
				category: "Development",
				linux: {
					install_method: "apt",
					install_command: "apt install test-app",
					official_support: true,
				},
				desktop_environments: ["gnome", "kde"],
			};

			const result = validateApplicationData(validData, "test.yaml");
			expect(result.errors).toEqual([]);
		});

		it("should fail validation for missing required fields", () => {
			const invalidData = {
				// Missing name and description
				category: "Development",
			};

			expect(() => {
				validateApplicationData(invalidData, "test.yaml");
			}).toThrow("Validation failed for test.yaml");
		});

		it("should warn about unknown categories", () => {
			const dataWithUnknownCategory = {
				name: "Test App",
				description: "Test description",
				category: "UnknownCategory",
			};

			const result = validateApplicationData(
				dataWithUnknownCategory,
				"test.yaml",
			);
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining("Validation warnings for test.yaml"),
			);
		});

		it("should detect dangerous install commands", () => {
			const dataWithDangerousCommand = {
				name: "Test App",
				description: "Test description",
				linux: {
					install_method: "manual",
					install_command: "rm -rf /", // Dangerous command
				},
			};

			expect(() => {
				validateApplicationData(dataWithDangerousCommand, "test.yaml");
			}).toThrow("Validation failed for test.yaml");
		});

		it("should warn about piped shell execution", () => {
			const dataWithPipedCommand = {
				name: "Test App",
				description: "Test description",
				linux: {
					install_method: "curlpipe",
					install_command: "curl -fsSL https://example.com/install.sh | bash",
				},
			};

			const result = validateApplicationData(dataWithPipedCommand, "test.yaml");
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining("Piped shell execution"),
			);
		});

		it("should validate install methods", () => {
			const dataWithInvalidMethod = {
				name: "Test App",
				description: "Test description",
				linux: {
					install_method: "invalid-method",
					install_command: "install test",
				},
			};

			const result = validateApplicationData(
				dataWithInvalidMethod,
				"test.yaml",
			);
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining('Unknown install method "invalid-method"'),
			);
		});

		it("should validate alternatives array", () => {
			const dataWithInvalidAlternatives = {
				name: "Test App",
				description: "Test description",
				linux: {
					install_method: "apt",
					install_command: "apt install test",
					alternatives: [
						{
							// Missing install_method
							install_command: "snap install test",
						},
					],
				},
			};

			const result = validateApplicationData(
				dataWithInvalidAlternatives,
				"test.yaml",
			);
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining("Alternative 0 missing install_method"),
			);
		});

		it("should handle null or invalid YAML structure", () => {
			expect(() => {
				validateApplicationData(null, "test.yaml");
			}).toThrow("Validation failed for test.yaml");

			expect(() => {
				validateApplicationData("not an object", "test.yaml");
			}).toThrow("Validation failed for test.yaml");
		});
	});

	describe("validatePluginData", () => {
		it("should pass validation for valid plugin data", () => {
			const validPlugin = {
				name: "test-plugin",
				description: "A test plugin for validation",
				type: "utility",
				priority: 50,
				status: "active",
				supports: { linux: true, macos: true },
				tags: ["test", "utility"],
			};

			const result = validatePluginData(validPlugin, 0);
			expect(result).toBe(true);
		});

		it("should fail validation for missing required fields", () => {
			const invalidPlugin = {
				// Missing name and description
				type: "utility",
			};

			const result = validatePluginData(invalidPlugin, 0);
			expect(result).toBeNull();
			expect(consoleSpy.error).toHaveBeenCalledWith(
				expect.stringContaining("Plugin validation errors"),
			);
		});

		it("should warn about invalid plugin names", () => {
			const pluginWithBadName = {
				name: "test plugin with spaces",
				description: "Valid description",
				type: "utility",
			};

			const result = validatePluginData(pluginWithBadName, 0);
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining("Name contains non-standard characters"),
			);
		});

		it("should validate priority range", () => {
			const pluginWithInvalidPriority = {
				name: "test-plugin",
				description: "Valid description",
				priority: 150, // Invalid: > 100
			};

			const result = validatePluginData(pluginWithInvalidPriority, 0);
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining("Priority should be a number between 0-100"),
			);
		});

		it("should validate status values", () => {
			const pluginWithInvalidStatus = {
				name: "test-plugin",
				description: "Valid description",
				status: "invalid-status",
			};

			const result = validatePluginData(pluginWithInvalidStatus, 0);
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining('Unknown status "invalid-status"'),
			);
		});

		it("should validate tags array", () => {
			const pluginWithInvalidTags = {
				name: "test-plugin",
				description: "Valid description",
				tags: ["valid", 123, "also-valid"], // 123 is not a string
			};

			const result = validatePluginData(pluginWithInvalidTags, 0);
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining("All tags should be strings"),
			);
		});

		it("should validate supports object type", () => {
			const pluginWithInvalidSupports = {
				name: "test-plugin",
				description: "Valid description",
				supports: "not an object",
			};

			const result = validatePluginData(pluginWithInvalidSupports, 0);
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining("Supports field should be an object"),
			);
		});

		it("should warn about long descriptions", () => {
			const pluginWithLongDescription = {
				name: "test-plugin",
				description: "A".repeat(350), // Very long description
				type: "utility",
			};

			const result = validatePluginData(pluginWithLongDescription, 0);
			expect(consoleSpy.warn).toHaveBeenCalledWith(
				expect.stringContaining("Description is very long"),
			);
		});
	});
});

describe("Integration Tests", () => {
	it("should have valid schema constants", () => {
		const scriptContent = fs.readFileSync(scriptPath, "utf8");

		// Check that required constants are defined
		expect(scriptContent).toContain("VALID_CATEGORIES");
		expect(scriptContent).toContain("VALID_INSTALL_METHODS");
		expect(scriptContent).toContain("VALID_DESKTOP_ENVIRONMENTS");

		// Check that common values are included
		expect(scriptContent).toContain("Development");
		expect(scriptContent).toContain("apt");
		expect(scriptContent).toContain("gnome");
	});

	it("should export generateToolsData function", () => {
		const scriptContent = fs.readFileSync(scriptPath, "utf8");
		expect(scriptContent).toContain("module.exports = { generateToolsData }");
	});
});
