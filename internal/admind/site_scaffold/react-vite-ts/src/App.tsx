import { useEffect, useState } from "react";
import { fallbackContent, loadSiteContent, type SiteContent } from "./site-content";

function sectionAnchor(index: number): string {
	return `section-${index + 1}`;
}

function App() {
	const [content, setContent] = useState<SiteContent>(fallbackContent);

	useEffect(() => {
		let isMounted = true;
		loadSiteContent().then((loadedContent) => {
			if (isMounted) setContent(loadedContent);
		});
		return () => {
			isMounted = false;
		};
	}, []);

	useEffect(() => {
		document.title = content.siteName;
	}, [content.siteName]);

	const { siteName, tagline, heroActionLabel, heroActionHref, sections } = content;
	const resolvedHeroActionHref = heroActionHref ?? (heroActionLabel ? "#section-1" : undefined);

	return (
		<div className="min-h-screen bg-background text-foreground antialiased">
			<header className="mx-auto flex max-w-4xl items-center justify-between px-6 py-6">
				<span className="text-sm font-semibold tracking-tight">{siteName}</span>
				<nav className="hidden gap-6 text-sm text-muted-foreground sm:flex">
					{sections.map((section, index) => (
						<a
							key={sectionAnchor(index)}
							href={"#" + sectionAnchor(index)}
							className="transition-colors hover:text-foreground"
						>
							{section.title}
						</a>
					))}
				</nav>
			</header>

			<main className="mx-auto max-w-4xl px-6">
				<section className="py-20">
					<h1 className="max-w-3xl text-5xl font-semibold leading-[1.05] tracking-tight">
						{siteName}
					</h1>
					<p className="mt-6 max-w-2xl text-lg leading-relaxed text-muted-foreground">{tagline}</p>
					{heroActionLabel ? (
						<a
							href={resolvedHeroActionHref}
							className="mt-8 inline-flex h-10 items-center rounded-md bg-primary px-5 text-sm font-medium text-primary-foreground transition-opacity hover:opacity-90"
						>
							{heroActionLabel}
						</a>
					) : null}
				</section>

				{sections.map((section, index) => (
					<section key={sectionAnchor(index)} id={sectionAnchor(index)} className="border-t border-border py-16">
						<h2 className="text-2xl font-semibold tracking-tight">{section.title}</h2>
						<p className="mt-4 max-w-2xl whitespace-pre-line leading-relaxed text-muted-foreground">
							{section.body}
						</p>
					</section>
				))}
			</main>

			<footer className="mx-auto max-w-4xl border-t border-border px-6 py-10 text-sm text-muted-foreground">
				<p>{siteName}</p>
			</footer>
		</div>
	);
}

export default App;
