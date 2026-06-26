<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import { Input } from '$lib/components/ui/input';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { appShellText } from '$lib/i18n/app-shell-text';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import LoaderIcon from '@lucide/svelte/icons/loader';

	type PublicAPITokenScope = 'read' | 'write' | 'destructive';

	type PublicAPITokenCreateResponse = {
		token: string;
		record?: {
			id: string;
			label: string;
			email: string;
			scopes: PublicAPITokenScope[];
			createdAt: string;
		};
	};

	let { open = $bindable(false) } = $props<{ open: boolean }>();

	const text = createPageText(appShellText);
	let tokenLabel = $state('Claude Code');
	let tokenScopes = $state<PublicAPITokenScope[]>(['read']);
	let tokenResult = $state<PublicAPITokenCreateResponse | null>(null);
	let message = $state('');
	let isCreating = $state(false);

	const tokenScopeOptions = (): { value: PublicAPITokenScope; label: string; description: string }[] => [
		{ value: 'read', label: text.apiTokenSheet.scopes.read, description: text.apiTokenSheet.scopeDescriptions.read },
		{ value: 'write', label: text.apiTokenSheet.scopes.write, description: text.apiTokenSheet.scopeDescriptions.write },
		{ value: 'destructive', label: text.apiTokenSheet.scopes.destructive, description: text.apiTokenSheet.scopeDescriptions.destructive }
	];

	function toggleTokenScope(scope: PublicAPITokenScope, isChecked: boolean) {
		if (scope === 'read') {
			tokenScopes = ['read', ...tokenScopes.filter((candidate) => candidate !== 'read')];
			return;
		}
		if (isChecked) {
			tokenScopes = [...new Set([...tokenScopes, scope])];
			return;
		}
		tokenScopes = tokenScopes.filter((candidate) => candidate !== scope);
	}

	async function createPublicAPIToken() {
		if (!tokenLabel.trim()) return;

		isCreating = true;
		message = '';
		tokenResult = null;
		try {
			const response = await fetch('/api/v1/tokens', {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					label: tokenLabel.trim(),
					scopes: tokenScopes
				})
			});
			if (!response.ok) {
				message = (await response.text()).trim() || text.apiTokenSheet.createError;
				return;
			}
			tokenResult = (await response.json()) as PublicAPITokenCreateResponse;
			message = text.apiTokenSheet.createSuccess;
		} catch {
			message = text.apiTokenSheet.createError;
		} finally {
			isCreating = false;
		}
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content class="w-full overflow-y-auto sm:max-w-md">
		<Sheet.Header>
			<div class="flex items-start justify-between gap-3 pr-8">
				<div>
					<Sheet.Title>{text.apiTokenSheet.title}</Sheet.Title>
					<Sheet.Description class="mt-1">{text.apiTokenSheet.description}</Sheet.Description>
				</div>
				<Badge variant="outline">/api/v1</Badge>
			</div>
		</Sheet.Header>
		<form
			class="grid gap-4 px-4"
			onsubmit={(event) => {
				event.preventDefault();
				createPublicAPIToken();
			}}
		>
			<label class="grid gap-1.5">
				<span class="text-xs font-medium text-muted-foreground">{text.apiTokenSheet.label}</span>
				<Input bind:value={tokenLabel} placeholder={text.apiTokenSheet.labelPlaceholder} autocomplete="off" />
			</label>
			<div class="grid gap-2">
				<p class="text-xs font-medium text-muted-foreground">{text.apiTokenSheet.scopesTitle}</p>
				<div class="grid gap-2">
					{#each tokenScopeOptions() as option (option.value)}
						<label class="flex min-w-0 gap-2 rounded-md border bg-background/70 p-3">
							<input
								class="mt-1 size-4 shrink-0"
								type="checkbox"
								checked={tokenScopes.includes(option.value)}
								disabled={option.value === 'read'}
								onchange={(event) => toggleTokenScope(option.value, event.currentTarget.checked)}
							/>
							<span class="min-w-0">
								<span class="block text-sm font-medium">{option.label}</span>
								<span class="mt-1 block text-xs leading-5 text-muted-foreground">{option.description}</span>
							</span>
						</label>
					{/each}
				</div>
			</div>
			<p class="text-xs text-muted-foreground">{text.apiTokenSheet.ownerBoundary}</p>
			<Button type="submit" disabled={isCreating || !tokenLabel.trim()} class="w-fit gap-2">
				{#if isCreating}
					<LoaderIcon class="size-4 animate-spin" />
				{/if}
				{text.apiTokenSheet.create}
			</Button>
		</form>
		<div class="grid gap-3 px-4 pb-4">
			{#if tokenResult?.token}
				<div class="grid gap-2 rounded-md border border-emerald-500/30 bg-emerald-500/10 p-3">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<p class="text-sm font-medium">{text.apiTokenSheet.createSuccess}</p>
						<CopyButton text={tokenResult.token} variant="outline" size="sm">
							{text.apiTokenSheet.copy}
						</CopyButton>
					</div>
					<code class="block max-w-full overflow-x-auto rounded bg-background px-2 py-1 text-xs">{tokenResult.token}</code>
					<p class="text-xs text-muted-foreground">{text.apiTokenSheet.shownOnce}</p>
				</div>
			{:else if message}
				<p class="rounded-md border bg-muted/30 px-3 py-2 text-sm">{message}</p>
			{/if}
		</div>
	</Sheet.Content>
</Sheet.Root>
