import type { Block } from "../site-content";
import { splitParagraphs } from "./textParagraphs";

type ProseProps = {
	block: Block;
	anchorID: string;
};

export function Prose({ block, anchorID }: ProseProps) {
	return (
		<section id={anchorID} className="border-t border-border py-16">
			{block.title ? <h2 className="text-2xl font-semibold tracking-tight">{block.title}</h2> : null}
			{splitParagraphs(block.body).map((paragraph, paragraphIndex) => (
				<p key={paragraphIndex} className="mt-4 max-w-2xl whitespace-pre-line leading-relaxed text-muted-foreground">
					{paragraph}
				</p>
			))}
		</section>
	);
}
