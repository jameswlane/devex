import Link from "next/link";

export default function NotFound() {
	return (
		<div className="min-h-screen flex items-center justify-center bg-gray-100">
			<div className="text-center">
				<h1 className="text-4xl font-bold text-gray-800 mb-4">
					404 - Page Not Found
				</h1>
				<p className="text-xl text-gray-600 mb-8">
					Oops! The page you're looking for doesn't exist.
				</p>
				<Link
					href="/"
					className="bg-blue-500 hover:bg-blue-600 text-white font-bold py-2 px-4 rounded"
				>
					Go back to homepage
				</Link>
			</div>
		</div>
	);
}
