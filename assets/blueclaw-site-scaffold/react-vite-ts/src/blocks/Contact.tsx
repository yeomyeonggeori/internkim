import { buttonVariants } from "../components/ui/button";
import type { Block } from "../site-content";
import { splitParagraphs } from "./textParagraphs";

type ContactProps = {
	block: Block;
	anchorID: string;
};

export function Contact({ block, anchorID }: ContactProps) {
	return (
		<section id={anchorID} className="py-20">
			<div className="surface-card px-8 py-12 sm:px-12">
				{block.title ? <h2 className="text-3xl font-bold tracking-tight">{block.title}</h2> : null}
				{splitParagraphs(block.body).map((paragraph, paragraphIndex) => (
					<p key={paragraphIndex} className="mt-4 max-w-2xl text-lg leading-relaxed text-muted-foreground">
						{paragraph}
					</p>
				))}
				{block.actionLabel ? (
					<a href={block.actionHref ?? "#"} className={buttonVariants({ size: "lg", className: "mt-8 px-8" })}>
						{block.actionLabel}
					</a>
				) : null}
			</div>
		</section>
	);
}
