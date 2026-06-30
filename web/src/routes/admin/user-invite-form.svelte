<script lang="ts">
	import { Button } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import MailIcon from '@lucide/svelte/icons/mail';
	import PlusIcon from '@lucide/svelte/icons/plus';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import type { AdminPageText, UserRole } from './admin-types';

	type UserRoleOption = {
		value: UserRole;
		label: string;
	};

	type UserInviteFormProps = {
		text: AdminPageText['users'];
		handle: string;
		name: string;
		email: string;
		hireDate: string;
		role: UserRole;
		roleOptions: UserRoleOption[];
		isSaving: boolean;
		isHandleValid: boolean;
		onSubmit: () => void;
	};

	let {
		text,
		handle = $bindable(),
		name = $bindable(),
		email = $bindable(),
		hireDate = $bindable(),
		role = $bindable(),
		roleOptions,
		isSaving,
		isHandleValid,
		onSubmit
	}: UserInviteFormProps = $props();
</script>

<Card.Root>
	<Card.Header class="gap-1">
		<Card.Title class="text-sm">{text.inviteTitle}</Card.Title>
		<Card.Description>{text.inviteDescription}</Card.Description>
	</Card.Header>
	<Card.Content>
		<form
			class="grid gap-3 lg:grid-cols-[minmax(120px,0.8fr)_minmax(150px,1fr)_minmax(210px,1.3fr)_150px_120px_auto] lg:items-end"
			onsubmit={(event) => {
				event.preventDefault();
				onSubmit();
			}}
		>
			<label class="grid gap-1.5">
				<Label>{text.handle}</Label>
				<Input bind:value={handle} placeholder="chanhee" autocomplete="off" />
			</label>
			<label class="grid gap-1.5">
				<Label>{text.realName}</Label>
				<Input bind:value={name} placeholder={text.realNamePlaceholder} autocomplete="off" />
			</label>
			<label class="grid gap-1.5">
				<Label>{text.email}</Label>
				<div class="relative">
					<MailIcon class="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
					<Input bind:value={email} type="email" placeholder={text.emailPlaceholder} class="pl-9" />
				</div>
			</label>
			<label class="grid gap-1.5">
				<Label>{text.hireDate}</Label>
				<Input bind:value={hireDate} type="date" />
			</label>
			<label class="grid gap-1.5">
				<Label>{text.role}</Label>
				<Select.Root type="single" bind:value={role}>
					<Select.Trigger class="w-full">
						{roleOptions.find((option) => option.value === role)?.label ?? '-'}
					</Select.Trigger>
					<Select.Content>
						{#each roleOptions as option (option.value)}
							<Select.Item value={option.value} label={option.label}>{option.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</label>
			<Button type="submit" disabled={isSaving || !email.trim() || !name.trim() || !isHandleValid} class="gap-2">
				{#if isSaving}
					<RefreshCwIcon class="size-4 animate-spin" />
				{:else}
					<PlusIcon class="size-4" />
				{/if}
				{text.invite}
			</Button>
		</form>
	</Card.Content>
</Card.Root>
