import { buttonVariants } from "../components/ui/button";
import type { Block } from "../site-content";
import { splitParagraphs } from "./textParagraphs";

type ContactProps = {
	block: Block;
	anchorID: string;
};

export function Contact({ block, anchorID }: ContactProps) {
	return (
		<section id={anchorID} className="border-t border-border py-16">
			{block.title ? <h2 className="text-2xl font-semibold tracking-tight">{block.title}</h2> : null}
			{splitParagraphs(block.body).map((paragraph, paragraphIndex) => (
				<p key={paragraphIndex} className="mt-4 max-w-2xl leading-relaxed text-muted-foreground">
					{paragraph}
				</p>
			))}
			{block.actionLabel ? (
				<a href={block.actionHref ?? "#"} className={buttonVariants({ size: "lg", className: "mt-6" })}>
					{block.actionLabel}
				</a>
			) : null}
		</section>
	);
}
