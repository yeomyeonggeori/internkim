<script lang="ts">
	import { periodLabel } from '$lib/period';
	import { dataRoomCompanyText } from './company-text';
	import { holderKinds, ownershipShares, totalShares, type CompanyOwnership, type HolderKind } from './company';

	let { ownership, locale }: { ownership: CompanyOwnership; locale: 'ko' | 'en' } = $props();

	const text = $derived(dataRoomCompanyText[locale].ownership);
	const shares = $derived(ownershipShares(ownership.kinds));
	const shownKinds = $derived(
		holderKinds.filter((kind) => [ownership.kinds, ...ownership.rounds.map((round) => round.kinds)].some((held) => held.some((entry) => entry.kind === kind)))
	);
	const percentFormat = $derived(new Intl.NumberFormat(locale, { maximumFractionDigits: 1, minimumFractionDigits: 1 }));
	const shareFormat = $derived(new Intl.NumberFormat(locale));
	const tones: Record<HolderKind, string> = {
		founder: 'bg-foreground/85',
		employee: 'bg-foreground/60',
		investor: 'bg-foreground/40',
		option_pool: 'bg-foreground/20',
		other: 'bg-foreground/10'
	};

	function percentIn(held: CompanyOwnership['kinds'], kind: HolderKind): string {
		const share = ownershipShares(held).find((entry) => entry.kind === kind);
		return share ? `${percentFormat.format(share.percent)}%` : '—';
	}
</script>

<section class="grid gap-3" aria-labelledby="company-ownership">
	<div class="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
		<h3 id="company-ownership" class="text-lg font-semibold">{text.title}</h3>
		<p class="text-muted-foreground text-xs tabular-nums">{text.asOf(ownership.asOf.replaceAll('-', '.'), shareFormat.format(totalShares(ownership.kinds)))}</p>
	</div>
	<div class="bg-border grid gap-px overflow-hidden rounded-lg border">
		<div class="bg-background grid gap-4 p-5">
			<div class="flex h-3 gap-0.5 overflow-hidden rounded-full" role="img" aria-label={shares.map((share) => `${text.kinds[share.kind]} ${percentFormat.format(share.percent)}%`).join(', ')}>
				{#each shares as share (share.kind)}
					<span class={tones[share.kind]} style:width={`${share.percent}%`}></span>
				{/each}
			</div>
			<dl class="grid grid-cols-2 gap-x-6 gap-y-3 sm:grid-cols-(--kind-columns)" style:--kind-columns={`repeat(${shares.length}, minmax(0, 1fr))`}>
				{#each shares as share (share.kind)}
					<div class="grid gap-0.5">
						<dt class="text-muted-foreground inline-flex items-center gap-1.5 text-xs"><span class="size-2 rounded-[2px] {tones[share.kind]}" aria-hidden="true"></span>{text.kinds[share.kind]}</dt>
						<dd class="text-xl font-semibold tracking-tight tabular-nums">{percentFormat.format(share.percent)}%</dd>
						<dd class="text-muted-foreground text-xs tabular-nums">{text.shares(shareFormat.format(share.shares))}</dd>
					</div>
				{/each}
			</dl>
		</div>
		{#if ownership.rounds.length > 0}
			<div class="bg-background overflow-x-auto">
				<table class="w-full min-w-[32rem] text-sm">
					<thead>
						<tr class="text-muted-foreground border-b text-xs">
							<th class="px-5 py-2.5 text-left font-normal">{text.afterRound}</th>
							{#each shownKinds as kind (kind)}<th class="px-3 py-2.5 text-right font-normal">{text.kinds[kind]}</th>{/each}
						</tr>
					</thead>
					<tbody class="divide-y">
						{#each ownership.rounds as round (round.title)}
							<tr>
								<td class="px-5 py-2.5">
									<span class="font-medium">{round.title}</span>
									{#if round.date}<span class="text-muted-foreground ml-2 text-xs tabular-nums">{periodLabel(round.date, locale)}</span>{/if}
								</td>
								{#each shownKinds as kind (kind)}<td class="px-3 py-2.5 text-right tabular-nums">{percentIn(round.kinds, kind)}</td>{/each}
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</section>
