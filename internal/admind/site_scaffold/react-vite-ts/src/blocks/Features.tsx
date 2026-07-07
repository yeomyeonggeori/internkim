import type { Block } from "../site-content";

type FeaturesProps = {
	block: Block;
	anchorID: string;
};

export function Features({ block, anchorID }: FeaturesProps) {
	const items = block.items ?? [];
	return (
		<section id={anchorID} className="py-20">
			{block.title ? <h2 className="text-3xl font-bold tracking-tight">{block.title}</h2> : null}
			{block.body ? <p className="mt-4 max-w-2xl text-lg leading-relaxed text-muted-foreground">{block.body}</p> : null}
			<div className="mt-10 grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
				{items.map((item, itemIndex) => (
					<div key={itemIndex} className="surface-card flex flex-col gap-3 p-6">
						<span className="item-index">{String(itemIndex + 1).padStart(2, "0")}</span>
						<h3 className="text-lg font-semibold leading-snug">{item.title}</h3>
						<p className="leading-relaxed text-muted-foreground">{item.body}</p>
					</div>
				))}
			</div>
		</section>
	);
}
