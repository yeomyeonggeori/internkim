<script lang="ts">
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import { CopyButton } from '$lib/components/ui/copy-button';
	import * as Field from '$lib/components/ui/field';
	import { Input } from '$lib/components/ui/input';
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

	const text = createPageText(appShellText);
	let tokenLabel = $state(todayTokenLabel());
	let tokenScopes = $state<PublicAPITokenScope[]>(['read']);
	let tokenResult = $state<PublicAPITokenCreateResponse | null>(null);
	let message = $state('');
	let isCreating = $state(false);

	function todayTokenLabel() {
		const today = new Date();
		const month = `${today.getMonth() + 1}`.padStart(2, '0');
		const day = `${today.getDate()}`.padStart(2, '0');
		return `${today.getFullYear()}-${month}-${day}`;
	}

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

<Card.Root>
	<Card.Header class="border-b pb-4">
		<Card.Title>{text.apiTokenSheet.title}</Card.Title>
		<Card.Description>{text.apiTokenSheet.description}</Card.Description>
		<Card.Action>
			<Badge variant="outline" class="font-mono">/api/v1</Badge>
		</Card.Action>
	</Card.Header>
	<form
		onsubmit={(event) => {
			event.preventDefault();
			createPublicAPIToken();
		}}
	>
		<Card.Content>
			<Field.Group class="max-w-xl">
				<Field.Field>
					<Field.Label for="api-token-label">{text.apiTokenSheet.label}</Field.Label>
					<Input id="api-token-label" bind:value={tokenLabel} autocomplete="off" />
				</Field.Field>
				<Field.Set>
					<Field.Legend variant="label">{text.apiTokenSheet.scopesTitle}</Field.Legend>
					<Field.Group data-slot="checkbox-group">
						{#each tokenScopeOptions() as option (option.value)}
							<Field.Label>
								<Field.Field orientation="horizontal">
									<Checkbox
										checked={tokenScopes.includes(option.value)}
										disabled={option.value === 'read'}
										onCheckedChange={(isChecked) => toggleTokenScope(option.value, isChecked)}
									/>
									<Field.Content>
										<Field.Title>{option.label}</Field.Title>
										<Field.Description>{option.description}</Field.Description>
									</Field.Content>
								</Field.Field>
							</Field.Label>
						{/each}
					</Field.Group>
					<Field.Description>{text.apiTokenSheet.ownerBoundary}</Field.Description>
				</Field.Set>
				{#if tokenResult?.token}
					<Field.Field>
						<Field.Label>{text.apiTokenSheet.createSuccess}</Field.Label>
						<div class="flex items-center gap-2">
							<code class="bg-muted min-w-0 flex-1 overflow-x-auto rounded-md border px-3 py-2 font-mono text-xs">{tokenResult.token}</code>
							<CopyButton text={tokenResult.token} variant="outline" />
						</div>
						<Field.Description>{text.apiTokenSheet.shownOnce}</Field.Description>
					</Field.Field>
				{:else if message}
					<Field.Error>{message}</Field.Error>
				{/if}
			</Field.Group>
		</Card.Content>
		<Card.Footer class="justify-end">
			<Button type="submit" disabled={isCreating || !tokenLabel.trim()}>
				{#if isCreating}
					<LoaderIcon class="size-4 animate-spin" />
				{/if}
				{text.apiTokenSheet.create}
			</Button>
		</Card.Footer>
	</form>
</Card.Root>
