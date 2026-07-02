import { useEffect, useState } from "react";
import { resolveBlockComponent } from "./blocks";
import { fallbackContent, loadSiteContent, type Block, type SiteContent } from "./site-content";

function blockAnchor(index: number): string {
	return `block-${index + 1}`;
}

type NavItem = {
	anchorID: string;
	title: string;
};

function navItemsFrom(blocks: Block[], siteName: string): NavItem[] {
	const seenTitles = new Set<string>();
	const navItems: NavItem[] = [];
	blocks.forEach((block, index) => {
		if (block.variant === "hero") return;
		if (!block.title || block.title === siteName) return;
		if (seenTitles.has(block.title)) return;
		seenTitles.add(block.title);
		navItems.push({ anchorID: blockAnchor(index), title: block.title });
	});
	return navItems;
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

	const { siteName, blocks } = content;
	const navItems = navItemsFrom(blocks, siteName);

	return (
		<div className="min-h-screen bg-background text-foreground antialiased">
			<header className="mx-auto flex max-w-4xl items-center justify-between px-6 py-6">
				<span className="text-sm font-semibold tracking-tight">{siteName}</span>
				<nav className="hidden gap-6 text-sm text-muted-foreground sm:flex">
					{navItems.map((navItem) => (
						<a
							key={navItem.anchorID}
							href={"#" + navItem.anchorID}
							className="transition-colors hover:text-foreground"
						>
							{navItem.title}
						</a>
					))}
				</nav>
			</header>

			<main className="mx-auto max-w-4xl px-6">
				{blocks.map((block, index) => {
					const BlockComponent = resolveBlockComponent(block.variant);
					return <BlockComponent key={blockAnchor(index)} block={block} anchorID={blockAnchor(index)} />;
				})}
			</main>

			<footer className="mx-auto max-w-4xl border-t border-border px-6 py-10 text-sm text-muted-foreground">
				<p>{siteName}</p>
			</footer>
		</div>
	);
}

export default App;
