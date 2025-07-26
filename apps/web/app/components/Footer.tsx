export function Footer() {
	return (
		<footer className="bg-gray-800 text-white py-8">
			<div className="container mx-auto px-4 text-center">
				<p>&copy; {new Date().getFullYear()} DevEx. All rights reserved.</p>
				<p className="mt-2">
					<a
						href="https://github.com/jameswlane/devex/"
						className="hover:underline"
					>
						View on GitHub
					</a>
				</p>
			</div>
		</footer>
	);
}
