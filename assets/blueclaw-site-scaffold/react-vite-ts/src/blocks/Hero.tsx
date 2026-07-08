import { buttonVariants } from "../components/ui/button";
import type { Block } from "../site-content";
import { splitParagraphs } from "./textParagraphs";

type HeroProps = {
	block: Block;
	anchorID: string;
};

export function Hero({ block, anchorID }: HeroProps) {
	return (
		<section id={anchorID} className="hero-band full-bleed px-6">
			<div className="mx-auto flex min-h-[52vh] max-w-4xl flex-col justify-center py-24">
				{block.title ? (
					<h1 className="max-w-3xl text-[clamp(2.75rem,6vw,4.25rem)] font-bold leading-[1.04] tracking-tight">
						{block.title}
					</h1>
				) : null}
				{splitParagraphs(block.body).map((paragraph, paragraphIndex) => (
					<p key={paragraphIndex} className="hero-muted mt-7 max-w-2xl text-xl leading-relaxed">
						{paragraph}
					</p>
				))}
				{block.actionLabel ? (
					<div className="mt-10">
						<a href={block.actionHref ?? "#"} className={buttonVariants({ size: "lg", className: "px-8 py-6 text-base" })}>
							{block.actionLabel}
						</a>
					</div>
				) : null}
			</div>
		</section>
	);
}
