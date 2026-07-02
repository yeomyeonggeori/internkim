import { ChevronDown } from "lucide-react";
import type { Block } from "../site-content";

type FAQProps = {
	block: Block;
	anchorID: string;
};

export function FAQ({ block, anchorID }: FAQProps) {
	const items = block.items ?? [];
	return (
		<section id={anchorID} className="border-t border-border py-16">
			{block.title ? <h2 className="text-2xl font-semibold tracking-tight">{block.title}</h2> : null}
			<div className="mt-6 divide-y divide-border rounded-lg border border-border">
				{items.map((item, itemIndex) => (
					<details key={itemIndex} className="group px-6 py-4">
						<summary className="flex cursor-pointer list-none items-center justify-between gap-4 font-medium [&::-webkit-details-marker]:hidden">
							{item.title}
							<ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground transition-transform group-open:rotate-180" />
						</summary>
						<p className="mt-3 leading-relaxed text-muted-foreground">{item.body}</p>
					</details>
				))}
			</div>
		</section>
	);
}
