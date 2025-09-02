package main

// getBuiltinRules returns the built-in technology detection rules
func getBuiltinRules() []DetectionRule {
	return []DetectionRule{
		// JavaScript/Node.js
		{
			Name:     "Node.js",
			Category: "runtime",
			Files: []FilePattern{
				{Path: "package.json", Confidence: 0.9},
				{Path: "package-lock.json", Confidence: 0.8},
				{Path: "yarn.lock", Confidence: 0.8},
				{Path: "pnpm-lock.yaml", Confidence: 0.8},
				{Path: ".nvmrc", Confidence: 0.7},
			},
			Directories: []DirectoryPattern{
				{Path: "node_modules", Confidence: 0.9},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "nodejs",
					Description: "Node.js runtime for JavaScript development",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "npm",
					Description: "Node.js package manager",
					Priority:    "critical",
				},
			},
		},
		// React
		{
			Name:     "React",
			Category: "frontend-framework",
			Content: []ContentPattern{
				{
					FilePattern:  "package\\.json",
					ContentRegex: "\"react\":",
					Description:  "React dependency found",
					Confidence:   0.9,
				},
				{
					FilePattern:  "\\.(js|jsx|ts|tsx)$",
					ContentRegex: "import.*react",
					Description:  "React import statements",
					Confidence:   0.8,
				},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "vscode",
					Description: "Visual Studio Code with React extensions",
					Priority:    "recommended",
				},
				{
					Type:        "environment",
					Action:      "configure",
					Target:      "eslint",
					Description: "ESLint for React code quality",
					Priority:    "recommended",
				},
			},
		},
		// Vue.js
		{
			Name:     "Vue.js",
			Category: "frontend-framework",
			Files: []FilePattern{
				{Path: "vue.config.js", Confidence: 0.9},
				{Pattern: "\\.vue$", Confidence: 0.8},
			},
			Content: []ContentPattern{
				{
					FilePattern:  "package\\.json",
					ContentRegex: "\"vue\":",
					Description:  "Vue.js dependency found",
					Confidence:   0.9,
				},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "vscode",
					Description: "Visual Studio Code with Vue extensions",
					Priority:    "recommended",
				},
			},
		},
		// Angular
		{
			Name:     "Angular",
			Category: "frontend-framework",
			Files: []FilePattern{
				{Path: "angular.json", Confidence: 0.95},
				{Path: "tsconfig.json", Confidence: 0.6},
			},
			Content: []ContentPattern{
				{
					FilePattern:  "package\\.json",
					ContentRegex: "\"@angular/core\":",
					Description:  "Angular dependency found",
					Confidence:   0.9,
				},
			},
			Directories: []DirectoryPattern{
				{Path: "src/app", Confidence: 0.7},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "angular-cli",
					Description: "Angular CLI for project management",
					Priority:    "critical",
				},
			},
		},
		// Python
		{
			Name:     "Python",
			Category: "language",
			Files: []FilePattern{
				{Path: "requirements.txt", Confidence: 0.8},
				{Path: "Pipfile", Confidence: 0.8},
				{Path: "pyproject.toml", Confidence: 0.8},
				{Path: "setup.py", Confidence: 0.7},
				{Path: ".python-version", Confidence: 0.7},
				{Pattern: "\\.py$", Confidence: 0.9},
			},
			Directories: []DirectoryPattern{
				{Path: "venv", Confidence: 0.6},
				{Path: ".venv", Confidence: 0.6},
				{Path: "__pycache__", Confidence: 0.8},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "python3",
					Description: "Python 3 interpreter",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "pip",
					Description: "Python package manager",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "virtualenv",
					Description: "Python virtual environment manager",
					Priority:    "recommended",
				},
			},
		},
		// Go
		{
			Name:     "Go",
			Category: "language",
			Files: []FilePattern{
				{Path: "go.mod", Confidence: 0.95},
				{Path: "go.sum", Confidence: 0.9},
				{Pattern: "\\.go$", Confidence: 0.9},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "go",
					Description: "Go programming language",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "vscode",
					Description: "Visual Studio Code with Go extensions",
					Priority:    "recommended",
				},
			},
		},
		// Rust
		{
			Name:     "Rust",
			Category: "language",
			Files: []FilePattern{
				{Path: "Cargo.toml", Confidence: 0.95},
				{Path: "Cargo.lock", Confidence: 0.9},
				{Pattern: "\\.rs$", Confidence: 0.9},
			},
			Directories: []DirectoryPattern{
				{Path: "src", Confidence: 0.6},
				{Path: "target", Confidence: 0.8},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "rust",
					Description: "Rust programming language",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "cargo",
					Description: "Rust package manager",
					Priority:    "critical",
				},
			},
		},
		// Java
		{
			Name:     "Java",
			Category: "language",
			Files: []FilePattern{
				{Path: "pom.xml", Confidence: 0.9},
				{Path: "build.gradle", Confidence: 0.9},
				{Path: "build.gradle.kts", Confidence: 0.9},
				{Pattern: "\\.java$", Confidence: 0.9},
			},
			Directories: []DirectoryPattern{
				{Path: "src/main/java", Confidence: 0.8},
				{Path: "target", Confidence: 0.6},
				{Path: "build", Confidence: 0.6},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "openjdk-17-jdk",
					Description: "OpenJDK Java Development Kit",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "maven",
					Description: "Apache Maven build tool",
					Priority:    "recommended",
				},
			},
		},
		// Docker
		{
			Name:     "Docker",
			Category: "containerization",
			Files: []FilePattern{
				{Path: "Dockerfile", Confidence: 0.95},
				{Path: "docker-compose.yml", Confidence: 0.9},
				{Path: "docker-compose.yaml", Confidence: 0.9},
				{Path: ".dockerignore", Confidence: 0.8},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "docker",
					Description: "Docker containerization platform",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "docker-compose",
					Description: "Docker Compose for multi-container applications",
					Priority:    "recommended",
				},
			},
		},
		// Kubernetes
		{
			Name:     "Kubernetes",
			Category: "orchestration",
			Files: []FilePattern{
				{Pattern: "k8s/.*\\.yaml$", Confidence: 0.8},
				{Pattern: "kubernetes/.*\\.yaml$", Confidence: 0.8},
				{Pattern: ".*-deployment\\.yaml$", Confidence: 0.7},
				{Pattern: ".*-service\\.yaml$", Confidence: 0.7},
			},
			Content: []ContentPattern{
				{
					FilePattern:  "\\.yaml$",
					ContentRegex: "apiVersion:.*/(v1|apps/v1)",
					Description:  "Kubernetes API version found",
					Confidence:   0.8,
				},
				{
					FilePattern:  "\\.yaml$",
					ContentRegex: "kind:\\s*(Deployment|Service|Pod|ConfigMap)",
					Description:  "Kubernetes resource kind found",
					Confidence:   0.9,
				},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "kubectl",
					Description: "Kubernetes command-line tool",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "minikube",
					Description: "Local Kubernetes development environment",
					Priority:    "recommended",
				},
			},
		},
		// Terraform
		{
			Name:     "Terraform",
			Category: "infrastructure",
			Files: []FilePattern{
				{Pattern: "\\.tf$", Confidence: 0.9},
				{Path: "terraform.tfvars", Confidence: 0.8},
				{Path: ".terraform.lock.hcl", Confidence: 0.9},
			},
			Directories: []DirectoryPattern{
				{Path: ".terraform", Confidence: 0.9},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "terraform",
					Description: "Terraform infrastructure as code tool",
					Priority:    "critical",
				},
			},
		},
		// Next.js
		{
			Name:     "Next.js",
			Category: "frontend-framework",
			Files: []FilePattern{
				{Path: "next.config.js", Confidence: 0.95},
				{Path: "next.config.mjs", Confidence: 0.95},
			},
			Content: []ContentPattern{
				{
					FilePattern:  "package\\.json",
					ContentRegex: "\"next\":",
					Description:  "Next.js dependency found",
					Confidence:   0.9,
				},
			},
			Directories: []DirectoryPattern{
				{Path: "pages", Confidence: 0.7},
				{Path: "app", Confidence: 0.6},
				{Path: ".next", Confidence: 0.8},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "nodejs",
					Description: "Node.js runtime for Next.js",
					Priority:    "critical",
				},
			},
		},
		// Django
		{
			Name:     "Django",
			Category: "backend-framework",
			Files: []FilePattern{
				{Path: "manage.py", Confidence: 0.9},
				{Path: "django.conf", Confidence: 0.8},
			},
			Content: []ContentPattern{
				{
					FilePattern:  "requirements\\.txt",
					ContentRegex: "Django",
					Description:  "Django dependency found",
					Confidence:   0.9,
				},
				{
					FilePattern:  "\\.py$",
					ContentRegex: "from django",
					Description:  "Django imports found",
					Confidence:   0.8,
				},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "python3",
					Description: "Python 3 for Django development",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "postgresql",
					Description: "PostgreSQL database for Django",
					Priority:    "recommended",
				},
			},
		},
		// Rails
		{
			Name:     "Ruby on Rails",
			Category: "backend-framework",
			Files: []FilePattern{
				{Path: "Gemfile", Confidence: 0.8},
				{Path: "Gemfile.lock", Confidence: 0.7},
				{Path: "config/application.rb", Confidence: 0.9},
				{Path: "Rakefile", Confidence: 0.6},
			},
			Content: []ContentPattern{
				{
					FilePattern:  "Gemfile",
					ContentRegex: "gem ['\"]rails['\"]",
					Description:  "Rails gem found",
					Confidence:   0.9,
				},
			},
			Directories: []DirectoryPattern{
				{Path: "app/models", Confidence: 0.8},
				{Path: "app/controllers", Confidence: 0.8},
				{Path: "config", Confidence: 0.6},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "ruby",
					Description: "Ruby programming language",
					Priority:    "critical",
				},
				{
					Type:        "application",
					Action:      "install",
					Target:      "bundler",
					Description: "Ruby dependency manager",
					Priority:    "critical",
				},
			},
		},
		// Flask
		{
			Name:     "Flask",
			Category: "backend-framework",
			Content: []ContentPattern{
				{
					FilePattern:  "requirements\\.txt",
					ContentRegex: "Flask",
					Description:  "Flask dependency found",
					Confidence:   0.9,
				},
				{
					FilePattern:  "\\.py$",
					ContentRegex: "from flask import",
					Description:  "Flask imports found",
					Confidence:   0.9,
				},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "python3",
					Description: "Python 3 for Flask development",
					Priority:    "critical",
				},
			},
		},
		// Git
		{
			Name:     "Git",
			Category: "version-control",
			Files: []FilePattern{
				{Path: ".gitignore", Confidence: 0.8},
				{Path: ".gitattributes", Confidence: 0.7},
			},
			Directories: []DirectoryPattern{
				{Path: ".git", Confidence: 0.95},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "git",
					Description: "Git version control system",
					Priority:    "critical",
				},
			},
		},
		// Database detection
		{
			Name:     "PostgreSQL",
			Category: "database",
			Content: []ContentPattern{
				{
					FilePattern:  "docker-compose\\.ya?ml",
					ContentRegex: "postgres",
					Description:  "PostgreSQL in Docker Compose",
					Confidence:   0.8,
				},
				{
					FilePattern:  "requirements\\.txt",
					ContentRegex: "psycopg2",
					Description:  "PostgreSQL Python driver",
					Confidence:   0.8,
				},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "postgresql",
					Description: "PostgreSQL database server",
					Priority:    "recommended",
				},
			},
		},
		{
			Name:     "MongoDB",
			Category: "database",
			Content: []ContentPattern{
				{
					FilePattern:  "docker-compose\\.ya?ml",
					ContentRegex: "mongo",
					Description:  "MongoDB in Docker Compose",
					Confidence:   0.8,
				},
				{
					FilePattern:  "package\\.json",
					ContentRegex: "\"mongoose\":",
					Description:  "Mongoose MongoDB driver",
					Confidence:   0.8,
				},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "application",
					Action:      "install",
					Target:      "mongodb",
					Description: "MongoDB database server",
					Priority:    "recommended",
				},
			},
		},
		// CI/CD
		{
			Name:     "GitHub Actions",
			Category: "cicd",
			Directories: []DirectoryPattern{
				{Path: ".github/workflows", Confidence: 0.95},
			},
			Files: []FilePattern{
				{Pattern: "\\.github/workflows/.*\\.ya?ml$", Confidence: 0.9},
			},
			Suggestions: []SuggestionRule{
				{
					Type:        "system",
					Action:      "configure",
					Target:      "github-actions",
					Description: "GitHub Actions CI/CD workflows",
					Priority:    "optional",
				},
			},
		},
	}
}
