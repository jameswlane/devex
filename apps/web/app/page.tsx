import { Footer } from "./components/Footer";
import { Header } from "./components/Header";
import { Hero } from "./components/Hero";
import { Resources } from "./components/Resources";
import { ToolSearch } from "./components/ToolSearch";

export default function Home() {
	return (
		<div className="min-h-screen bg-gray-100">
			<Header />
			<main className="container mx-auto px-4 py-8">
				<Hero />
				<ToolSearch />
				<Resources />
			</main>
			<Footer />
		</div>
	);
}
