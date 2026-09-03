<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Separator } from '$lib/components/ui/separator';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import { onMount } from 'svelte';
	import { fetchSkillInventory, skillRootsOf, type SkillInventory } from './skills-api';
	import { skillsText } from './skills-text';

	const text = createPageText(skillsText);
	let inventory = $state<SkillInventory | undefined>(undefined);
	let loadError = $state('');
	let isLoading = $state(false);

	const skillRoots = $derived(inventory ? skillRootsOf(inventory.skills) : []);

	async function load() {
		isLoading = true;
		loadError = '';
		try {
			inventory = await fetchSkillInventory();
		} catch {
			loadError = text.loadError;
		} finally {
			isLoading = false;
		}
	}

	function countLabel(template: string, count: number): string {
		return template.replace('{count}', String(count));
	}

	onMount(load);
</script>

<Card.Root>
	<Card.Header class="gap-3">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<Card.Title>{text.title}</Card.Title>
			<div class="flex items-center gap-2">
				{#if inventory}
					<Badge variant="secondary">{countLabel(text.loadedCount, inventory.skills.length)}</Badge>
					{#if inventory.unavailableSkills.length > 0}
						<Badge variant="outline">{countLabel(text.unavailableCount, inventory.unavailableSkills.length)}</Badge>
					{/if}
				{/if}
				<Button variant="outline" size="sm" disabled={isLoading} onclick={load}>
					<RefreshCwIcon data-icon="inline-start" />
					{text.refresh}
				</Button>
			</div>
		</div>
		<Card.Description>{text.description}</Card.Description>
	</Card.Header>
	<Card.Content class="flex flex-col gap-4">
		{#if loadError}
			<p class="text-sm text-destructive">{loadError}</p>
		{:else if !inventory}
			<Skeleton class="h-24 w-full" />
		{:else if inventory.skills.length === 0 && inventory.unavailableSkills.length === 0}
			<p class="text-sm text-muted-foreground">{text.empty}</p>
		{:else}
			{#each skillRoots as skillRoot (skillRoot.path)}
				<div class="flex flex-col gap-2">
					<div class="flex flex-wrap items-baseline gap-2">
						<span class="text-xs text-muted-foreground">{text.rootLabel}</span>
						<code class="rounded-md bg-muted px-2 py-0.5 text-xs">{skillRoot.path}</code>
					</div>
					<ul class="grid gap-2">
						{#each skillRoot.skills as skill (skill.path)}
							<li class="rounded-lg border px-3 py-2">
								<div class="flex flex-wrap items-center gap-2">
									<span class="text-sm font-medium">{skill.name}</span>
									{#each skill.toolReferences as toolName (toolName)}
										<Badge variant="outline">{toolName}</Badge>
									{/each}
								</div>
								{#if skill.description}
									<p class="mt-1 text-sm text-muted-foreground">{skill.description}</p>
								{/if}
							</li>
						{/each}
					</ul>
				</div>
			{/each}

			{#if inventory.unavailableSkills.length > 0}
				<Separator />
				<div class="flex flex-col gap-2">
					<div class="flex items-center gap-2">
						<TriangleAlertIcon class="size-4 text-muted-foreground" />
						<span class="text-sm font-medium">{text.unavailableTitle}</span>
					</div>
					<p class="text-sm text-muted-foreground">{text.unavailableDescription}</p>
					<ul class="grid gap-2">
						{#each inventory.unavailableSkills as skill (skill.path)}
							<li class="rounded-lg border border-dashed px-3 py-2">
								<span class="text-sm font-medium">{skill.name}</span>
								{#if skill.missingEnvironmentVariables.length > 0}
									<p class="mt-1 text-xs text-muted-foreground">
										{text.missingEnvironment}: {skill.missingEnvironmentVariables.join(', ')}
									</p>
								{/if}
								{#if skill.missingToolNames.length > 0}
									<p class="mt-1 text-xs text-muted-foreground">
										{text.missingTools}: {skill.missingToolNames.join(', ')}
									</p>
								{/if}
							</li>
						{/each}
					</ul>
				</div>
			{/if}
		{/if}
	</Card.Content>
</Card.Root>
