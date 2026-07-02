import { buttonVariants } from "../components/ui/button";
import type { Block } from "../site-content";
import { splitParagraphs } from "./textParagraphs";

type HeroProps = {
	block: Block;
	anchorID: string;
};

export function Hero({ block, anchorID }: HeroProps) {
	return (
		<section id={anchorID} className="pt-20 pb-12">
			{block.title ? (
				<h1 className="max-w-3xl text-5xl font-semibold leading-[1.05] tracking-tight">{block.title}</h1>
			) : null}
			{splitParagraphs(block.body).map((paragraph, paragraphIndex) => (
				<p key={paragraphIndex} className="mt-6 max-w-2xl text-lg leading-relaxed text-muted-foreground">
					{paragraph}
				</p>
			))}
			{block.actionLabel ? (
				<a href={block.actionHref ?? "#"} className={buttonVariants({ size: "lg", className: "mt-8" })}>
					{block.actionLabel}
				</a>
			) : null}
		</section>
	);
}
