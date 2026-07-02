import { buttonVariants } from "../components/ui/button";
import type { Block } from "../site-content";

type CTAProps = {
	block: Block;
	anchorID: string;
};

export function CTA({ block, anchorID }: CTAProps) {
	return (
		<section id={anchorID} className="my-16 rounded-lg bg-primary px-8 py-12 text-primary-foreground">
			{block.title ? <h2 className="text-3xl font-semibold tracking-tight">{block.title}</h2> : null}
			{block.body ? <p className="mt-3 max-w-2xl leading-relaxed opacity-80">{block.body}</p> : null}
			{block.actionLabel ? (
				<a
					href={block.actionHref ?? "#"}
					className={buttonVariants({ variant: "secondary", size: "lg", className: "mt-6" })}
				>
					{block.actionLabel}
				</a>
			) : null}
		</section>
	);
}
