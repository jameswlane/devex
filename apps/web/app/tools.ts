export interface Tool {
	name: string;
	description: string;
	categories: string[];
	gitUrl?: string;
	websiteUrl?: string;
}

export const tools: Tool[] = [
	{
		name: "bat",
		description: "A cat clone with syntax highlighting and Git integration.",
		categories: ["Command Line", "Productivity"],
		gitUrl: "https://github.com/sharkdp/bat",
		websiteUrl: "https://github.com/sharkdp/bat",
	},
	{
		name: "btop",
		description:
			"A resource monitor that shows usage and stats for processor, memory, disks, network and processes.",
		categories: ["Monitoring", "System"],
		gitUrl: "https://github.com/aristocratos/btop",
		websiteUrl: "https://github.com/aristocratos/btop",
	},
	{
		name: "Docker",
		description:
			"Platform for developing, shipping, and running applications in containers.",
		categories: ["Containerization", "DevOps"],
		gitUrl: "https://github.com/docker/docker-ce",
		websiteUrl: "https://www.docker.com/",
	},
	{
		name: "eza",
		description: "A modern, improved version of the `ls` command.",
		categories: ["Command Line", "Productivity"],
		gitUrl: "https://github.com/eza-community/eza",
		websiteUrl: "https://github.com/eza-community/eza",
	},
	{
		name: "Fastfetch",
		description:
			"A fast and highly customizable system information script for Linux.",
		categories: ["System Information", "Linux"],
		gitUrl: "https://github.com/LinusDierheimer/fastfetch",
		websiteUrl: "https://github.com/LinusDierheimer/fastfetch",
	},
	{
		name: "fd-find",
		description: "A simple, fast and user-friendly alternative to find.",
		categories: ["Search", "Productivity"],
		gitUrl: "https://github.com/sharkdp/fd",
		websiteUrl: "https://github.com/sharkdp/fd",
	},
	{
		name: "Firefox Developer Edition",
		description:
			"A version of Firefox tailored for web developers, with tools and features to make development easier.",
		categories: ["Browser", "Development Tool"],
		gitUrl: "https://github.com/mozilla/gecko-dev",
		websiteUrl: "https://www.mozilla.org/firefox/developer/",
	},
	{
		name: "Flatpak",
		description:
			"A system for building, distributing, and running sandboxed desktop applications on Linux.",
		categories: ["Package Management", "Linux"],
		gitUrl: "https://github.com/flatpak/flatpak",
		websiteUrl: "https://flatpak.org/",
	},
	{
		name: "Flameshot",
		description: "A powerful yet simple to use screenshot software.",
		categories: ["Screenshot", "Productivity"],
		gitUrl: "https://github.com/flameshot-org/flameshot",
		websiteUrl: "https://flameshot.org/",
	},
	{
		name: "fzf",
		description: "A general-purpose command-line fuzzy finder.",
		categories: ["Search", "Productivity"],
		gitUrl: "https://github.com/junegunn/fzf",
		websiteUrl: "https://github.com/junegunn/fzf",
	},
	{
		name: "Git",
		description:
			"Distributed version control system for tracking changes in source code during software development.",
		categories: ["Version Control", "Development Tool"],
		gitUrl: "https://github.com/git/git",
		websiteUrl: "https://git-scm.com/",
	},
	{
		name: "GitHub CLI",
		description:
			"A command-line tool for GitHub, enabling you to manage your GitHub repositories and workflows directly from your terminal.",
		categories: ["Version Control", "Productivity"],
		gitUrl: "https://github.com/cli/cli",
		websiteUrl: "https://cli.github.com/",
	},
	{
		name: "Gnome Tweaks",
		description: "A tool to adjust advanced GNOME options.",
		categories: ["Customization", "Linux"],
		gitUrl: "https://gitlab.gnome.org/GNOME/gnome-tweaks",
		websiteUrl: "https://wiki.gnome.org/Apps/Tweaks",
	},
	{
		name: "JetBrains Toolbox",
		description:
			"A single access point for managing your JetBrains tools and projects.",
		categories: ["Productivity", "Development Tool"],
		gitUrl: "https://github.com/JetBrains/toolbox-app",
		websiteUrl: "https://www.jetbrains.com/toolbox-app/",
	},
	{
		name: "lazydocker",
		description:
			"A simple terminal UI for Docker commands, written in Go with the gocui library.",
		categories: ["Containerization", "Productivity"],
		gitUrl: "https://github.com/jesseduffield/lazydocker",
		websiteUrl: "https://github.com/jesseduffield/lazydocker",
	},
	{
		name: "lazygit",
		description:
			"A simple terminal UI for git commands, written in Go with the gocui library.",
		categories: ["Version Control", "Productivity"],
		gitUrl: "https://github.com/jesseduffield/lazygit",
		websiteUrl: "https://github.com/jesseduffield/lazygit",
	},
	{
		name: "LocalSend",
		description: "An open-source cross-platform app for sharing files locally.",
		categories: ["File Sharing", "Cross-Platform"],
		gitUrl: "https://github.com/localsend/localsend",
		websiteUrl: "https://localsend.org/",
	},
	{
		name: "mise",
		description:
			"mise is a tool that manages installations of programming language runtimes and other tools for local development",
		categories: ["Development Tool", "Productivity"],
		gitUrl: "https://github.com/jdx/mise",
		websiteUrl: "https://mise.jdx.dev/",
	},
	{
		name: "nala",
		description:
			"A front-end for libapt-pkg, providing a better user experience for APT package management.",
		categories: ["Package Management", "Linux"],
		gitUrl: "https://github.com/volitank/nala",
		websiteUrl: "https://gitlab.com/volian/nala",
	},
	{
		name: "Neovim",
		description: "A hyperextensible Vim-based text editor.",
		categories: ["Editor", "Productivity"],
		gitUrl: "https://github.com/neovim/neovim",
		websiteUrl: "https://neovim.io/",
	},
	{
		name: "plocate",
		description:
			"A much faster locate, intended to be a drop-in replacement for mlocate.",
		categories: ["Search", "Productivity"],
		gitUrl: "https://github.com/plocate/plocate",
		websiteUrl: "https://plocate.sesse.net/",
	},
	{
		name: "Podman",
		description:
			"A daemonless container engine for developing, managing, and running OCI Containers on your Linux System.",
		categories: ["Containerization", "DevOps"],
		gitUrl: "https://github.com/containers/podman",
		websiteUrl: "https://podman.io/",
	},
	{
		name: "pre-commit",
		description:
			"A framework for managing and maintaining multi-language pre-commit hooks.",
		categories: ["Version Control", "Productivity"],
		gitUrl: "https://github.com/pre-commit/pre-commit",
		websiteUrl: "https://pre-commit.com/",
	},
	{
		name: "ripgrep",
		description:
			"A line-oriented search tool that recursively searches your current directory for a regex pattern.",
		categories: ["Search", "Productivity"],
		gitUrl: "https://github.com/BurntSushi/ripgrep",
		websiteUrl: "https://github.com/BurntSushi/ripgrep",
	},
	{
		name: "Typora",
		description:
			"A minimal markdown editor, providing seamless experience as both reader and writer.",
		categories: ["Editor", "Markdown"],
		gitUrl: "https://github.com/typora/typora-issues",
		websiteUrl: "https://typora.io/",
	},
	{
		name: "Ulauncher",
		description: "A fast application launcher for Linux.",
		categories: ["Productivity", "Linux"],
		gitUrl: "https://github.com/Ulauncher/Ulauncher",
		websiteUrl: "https://ulauncher.io/",
	},
	{
		name: "VLC",
		description:
			"A free and open-source cross-platform multimedia player that plays most multimedia files, as well as DVDs, audio CDs, VCDs, and various streaming protocols.",
		categories: ["Media Player", "Cross-Platform"],
		gitUrl: "https://github.com/videolan/vlc",
		websiteUrl: "https://www.videolan.org/vlc/",
	},
	{
		name: "zoxide",
		description: "A smarter cd command, inspired by z and autojump.",
		categories: ["Productivity", "Shell"],
		gitUrl: "https://github.com/ajeetdsouza/zoxide",
		websiteUrl: "https://github.com/ajeetdsouza/zoxide",
	},
];
