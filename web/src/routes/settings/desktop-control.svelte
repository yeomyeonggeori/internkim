<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import {
		desktopControlShell,
		type DesktopControlPlugin,
		type DesktopControlState
	} from '$lib/native-shell/desktop-control';
	import { shellPlatform } from '$lib/native-shell/shell';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { companySettingsText } from './text';

	const text = createPageText(companySettingsText);
	const fieldID = $props.id();

	let shell = $state<DesktopControlPlugin | null>(null);
	let control = $state<DesktopControlState>({ isAllowed: false, accessibility: null });
	let isSwitching = $state(false);
	let isMac = $state(false);

	const needsAccessibility = $derived(isMac && control.isAllowed && control.accessibility !== 'granted');

	onMount(() => {
		let stopFollowing = () => {};
		let isDisposed = false;
		desktopControlShell().then(async (found) => {
			if (!found || isDisposed) return;
			shell = found;
			isMac = shellPlatform() === 'macos';
			stopFollowing = await found.onChanged(() => void refresh());
			await refresh();
		}, () => toast.error(text.desktopControlFailed));
		return () => {
			isDisposed = true;
			stopFollowing();
		};
	});

	async function refresh() {
		if (!shell) return;
		control = await shell.state();
	}

	async function choose(wanted: boolean) {
		if (!shell) return;
		isSwitching = true;
		try {
			control = wanted ? await shell.allow(currentLocale.value) : await shell.disallow();
		} catch {
			toast.error(text.desktopControlFailed);
		} finally {
			isSwitching = false;
		}
	}

	async function openPermissionGuide() {
		if (!shell) return;
		const answer = await shell.openPermissionGuide();
		if ('refusal' in answer) toast.error(answer.refusal);
	}
</script>

<svelte:window onfocus={() => void refresh()} />

{#if shell}
	<Card.Root>
		<Card.Header>
			<Card.Title>{text.desktopControl}</Card.Title>
			<Card.Description>{text.desktopControlDescription}</Card.Description>
		</Card.Header>
		<Card.Content class="grid gap-3">
			<div class="flex items-center justify-between gap-4">
				<Label for="{fieldID}-desktop-control" class="text-sm font-normal">{text.desktopControlSwitch}</Label>
				<Switch
					id="{fieldID}-desktop-control"
					bind:checked={() => control.isAllowed, (wanted) => void choose(wanted)}
					disabled={isSwitching}
				/>
			</div>
			{#if needsAccessibility}
				<div class="flex items-center justify-between gap-4">
					<p class="text-sm text-muted-foreground">{text.desktopControlNeedsAccessibility}</p>
					<Button variant="outline" size="sm" onclick={openPermissionGuide}>
						{text.desktopControlGrantAccessibility}
					</Button>
				</div>
			{/if}
		</Card.Content>
	</Card.Root>
{/if}
