import { Card, CardContent, CardHeader, CardTitle } from "../components/ui/card";
import type { Block } from "../site-content";

type FeaturesProps = {
	block: Block;
	anchorID: string;
};

export function Features({ block, anchorID }: FeaturesProps) {
	const items = block.items ?? [];
	return (
		<section id={anchorID} className="border-t border-border py-16">
			{block.title ? <h2 className="text-2xl font-semibold tracking-tight">{block.title}</h2> : null}
			{block.body ? <p className="mt-4 max-w-2xl leading-relaxed text-muted-foreground">{block.body}</p> : null}
			<div className="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
				{items.map((item, itemIndex) => (
					<Card key={itemIndex}>
						<CardHeader>
							<CardTitle className="text-lg">{item.title}</CardTitle>
						</CardHeader>
						<CardContent>
							<p className="leading-relaxed text-muted-foreground">{item.body}</p>
						</CardContent>
					</Card>
				))}
			</div>
		</section>
	);
}
