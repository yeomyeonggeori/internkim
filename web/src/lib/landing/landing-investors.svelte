<script lang="ts">
	import Building2Icon from '@lucide/svelte/icons/building-2';
	import LockKeyholeIcon from '@lucide/svelte/icons/lock-keyhole';
	import UserRoundSearchIcon from '@lucide/svelte/icons/user-round-search';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LandingCircleScopes from './landing-circle-scopes.svelte';
	import { landingText } from './text';

	const text = createPageText(landingText);
	const audiences = $derived([
		{ icon: Building2Icon, title: text.investors.company.title, items: text.investors.company.items },
		{ icon: UserRoundSearchIcon, title: text.investors.investor.title, items: text.investors.investor.items }
	]);
</script>

<div class="grid gap-10 lg:grid-cols-[minmax(0,6fr)_minmax(0,5fr)] lg:gap-12">
	<div class="grid content-start gap-10 sm:grid-cols-2 sm:gap-8">
		{#each audiences as audience (audience.title)}
			<section class="grid content-start gap-6">
				<h3 class="text-muted-foreground flex items-center gap-2 text-sm font-medium">
					<audience.icon class="size-4" />
					{audience.title}
				</h3>
				<ul class="grid gap-6">
					{#each audience.items as item (item.title)}
						<li class="grid gap-1.5">
							<p class="font-semibold break-keep">{item.title}</p>
							<p class="text-muted-foreground leading-7 break-keep">{item.body}</p>
						</li>
					{/each}
				</ul>
			</section>
		{/each}
	</div>
	<div class="grid content-start gap-4">
		<LandingCircleScopes title={text.investors.scope} />
		<p class="text-muted-foreground flex gap-2 text-sm leading-6 break-keep">
			<LockKeyholeIcon class="mt-1 size-3.5 shrink-0" />
			{text.investors.guard}
		</p>
	</div>
</div>
