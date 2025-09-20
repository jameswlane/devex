import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
	title: "DevEx Registry - Official DevEx CLI Registry",
	description:
		"The official registry for DevEx CLI applications, plugins, configs, and stacks. Discover and manage development tools across Linux, macOS, and Windows.",
	keywords:
		"devex, cli, registry, applications, plugins, development tools, linux, macos, windows",
	authors: [{ name: "DevEx Team" }],
	creator: "DevEx Team",
	publisher: "DevEx",
	formatDetection: {
		email: false,
		address: false,
		telephone: false,
	},
	openGraph: {
		type: "website",
		locale: "en_US",
		url: "https://registry.devex.sh",
		title: "DevEx Registry",
		description:
			"The official registry for DevEx CLI applications, plugins, configs, and stacks.",
		siteName: "DevEx Registry",
	},
	twitter: {
		card: "summary_large_image",
		title: "DevEx Registry",
		description:
			"The official registry for DevEx CLI applications, plugins, configs, and stacks.",
	},
	robots: {
		index: false,
		follow: false,
		googleBot: {
			index: false,
			follow: false,
			"max-video-preview": -1,
			"max-image-preview": "large",
			"max-snippet": -1,
		},
	},
};

export default function RootLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<html lang="en">
			<head>
				<link rel="icon" href="/favicon.ico" />
				<link rel="canonical" href="https://registry.devex.sh" />
				<meta name="robots" content="noindex, nofollow" />
				<meta name="googlebot" content="noindex, nofollow" />
			</head>
			<body className="antialiased">{children}</body>
		</html>
	);
}
