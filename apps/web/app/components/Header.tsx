import Image from "next/image";
import Link from "next/link";

export function Header() {
	return (
		<header className="bg-white shadow-md">
			<div className="container mx-auto px-4 py-4 flex items-center justify-between">
				<Link href="/" className="flex items-center space-x-2">
					<Image
						src="/assets/devex.svg"
						alt="DevEx Logo"
						width={40}
						height={40}
					/>
					<span className="text-2xl font-bold text-gray-800">DevEx</span>
				</Link>
				<nav>
					<ul className="flex space-x-4">
						<li>
							<Link href="#tools" className="text-gray-600 hover:text-gray-800">
								Tools
							</Link>
						</li>
						<li>
							<Link
								href="#resources"
								className="text-gray-600 hover:text-gray-800"
							>
								Resources
							</Link>
						</li>
						<li>
							<a
								href="https://github.com/jameswlane/devex/"
								className="text-gray-600 hover:text-gray-800"
							>
								GitHub
							</a>
						</li>
					</ul>
				</nav>
			</div>
		</header>
	);
}
