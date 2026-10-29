/** @type {import('next').NextConfig} */
const nextConfig = {
	images: {
		unoptimized: true,
	},
	async redirects() {
		return [
			{
				source: "/install",
				destination: "https://get.devex.sh/install",
				permanent: false,
			},
			{
				source: "/install.ps1",
				destination: "https://get.devex.sh/install.ps1",
				permanent: false,
			},
		];
	},
};

module.exports = nextConfig;
