import "./globals.css";
import { Analytics } from "@vercel/analytics/next";
import { SpeedInsights } from "@vercel/speed-insights/next";
import type { Metadata } from "next";
import { Inter } from "next/font/google";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
	title: "DevEx - Development Environment Setup CLI",
	description:
		"Easily setup your development environment for Linux and macOS with DevEx CLI",
	icons: {
		icon: [
			{
				url: "/assets/favicon.webp",
				rel: "icon",
				type: "image/webp",
			},
		],
	},
};

export default function RootLayout({
	children,
}: {
	children: React.ReactNode;
}) {
	return (
		<html lang="en">
			<body className={inter.className}>
				{children}
				<Analytics />
				<SpeedInsights />
			</body>
		</html>
	);
}
