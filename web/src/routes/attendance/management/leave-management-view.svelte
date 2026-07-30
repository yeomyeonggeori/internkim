<script lang="ts">
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { Badge } from '$lib/components/ui/badge';
	import { Button, buttonVariants } from '$lib/components/ui/button';
	import * as Card from '$lib/components/ui/card';
	import { Input } from '$lib/components/ui/input';
	import PersonAvatar from '$lib/components/person-avatar.svelte';
	import * as Table from '$lib/components/ui/table';
	import { localizedLeaveTypeName } from '$lib/i18n/leave-type-name';
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { createPageText } from '$lib/i18n/page-text.svelte';
	import RefreshCwIcon from '@lucide/svelte/icons/refresh-cw';
	import { milliDaysValue } from '../leave/leave-history-model';
	import { attendanceText } from '../text';
	import LeaveAdjustmentDialog from './leave-adjustment-dialog.svelte';
	import { getLeaveManagementState } from './leave-management-state.svelte';
	import LeaveTimeCorrectionDialog from './leave-time-correction-dialog.svelte';
	import PastLeaveDialog from './past-leave-dialog.svelte';

	const text = createPageText(attendanceText);
	const management = getLeaveManagementState();
	let search = $state('');

	const filteredEmployees = $derived(
		(management.payload?.employees ?? []).filter((employee) => {
			const query = search.trim().toLowerCase();
			return (
				!query ||
				employee.displayName.toLowerCase().includes(query) ||
				employee.email.toLowerCase().includes(query)
			);
		})
	);

	function dayValue(value: number): string {
		return `${milliDaysValue(value)}${text.management.dayUnit}`;
	}

	function signedDayValue(value: number): string {
		return `${value > 0 ? '+' : ''}${dayValue(value)}`;
	}

	function operationLabel(operationType: string): string {
		if (operationType === 'legalCorrection') return text.management.legalCorrection;
		if (operationType === 'adjustment') return text.management.manualAdjustment;
		if (operationType === 'grant') return text.management.grant;
		if (operationType === 'reserve') return text.management.reserve;
		if (operationType === 'release') return text.management.release;
		if (operationType === 'use' || operationType === 'untrackedUse') {
			return text.management.use;
		}
		if (operationType === 'expire') return text.management.expire;
		return text.management.otherChange;
	}

	function requestStatus(status: string): string {
		if (status === 'pending') return text.management.pending;
		if (status === 'approved') return text.management.approved;
		if (status === 'rejected') return text.management.rejected;
		if (status === 'cancelled') return text.management.cancelled;
		return text.management.needsChanges;
	}

	function leaveTypeName(id: string, name: string): string {
		return localizedLeaveTypeName(id, name, currentLocale.value);
	}
</script>

<section class="mx-auto min-h-0 w-full max-w-7xl space-y-5" data-testid="leave-management-view">
	<header class="flex flex-wrap items-start justify-between gap-4">
		<div>
			<h1 class="text-2xl font-semibold tracking-tight">{text.management.title}</h1>
			<p class="mt-1 text-sm text-muted-foreground">{text.management.description}</p>
		</div>
		<div class="flex flex-wrap gap-2">
			<LeaveAdjustmentDialog />
			<PastLeaveDialog />
			<Button
				variant={management.isLoading ? 'secondary' : 'ghost'}
				size="icon"
				aria-label={text.management.refresh}
				aria-busy={management.isLoading}
				onclick={() => management.load()}
				disabled={management.isLoading}
				data-testid="leave-management-refresh"
			>
				<RefreshCwIcon class={management.isLoading ? 'animate-spin text-primary' : ''} />
			</Button>
		</div>
	</header>

	{#if management.errorMessage}
		<p class="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive">
			{management.errorMessage}
		</p>
	{/if}

	<div class="grid min-h-0 gap-5 xl:grid-cols-[minmax(28rem,1fr)_minmax(30rem,1.2fr)]">
		<Card.Root class="min-h-0">
			<Card.Header class="gap-3">
				<div>
					<Card.Title>{text.management.employeeList}</Card.Title>
					<Card.Description>{text.management.employeeListDescription}</Card.Description>
				</div>
				<Input bind:value={search} placeholder={text.management.searchPlaceholder} />
			</Card.Header>
			<Card.Content class="min-h-0 overflow-x-auto">
				<Table.Root>
					<Table.Header>
						<Table.Row>
							<Table.Head>{text.management.employee}</Table.Head>
							<Table.Head class="text-right">{text.management.granted}</Table.Head>
							<Table.Head class="text-right">{text.management.used}</Table.Head>
							<Table.Head class="text-right">{text.management.pending}</Table.Head>
							<Table.Head class="text-right">{text.management.available}</Table.Head>
							<Table.Head class="text-right">{text.management.expiring}</Table.Head>
						</Table.Row>
					</Table.Header>
					<Table.Body>
						{#each filteredEmployees as employee (employee.email)}
							{@const isSelected = management.selectedEmployeeEmail === employee.email}
							<Table.Row
								data-state={isSelected ? 'selected' : undefined}
								class={isSelected ? 'bg-primary/10' : ''}
							>
								<Table.Cell>
									<button
										type="button"
										class="flex items-center gap-2 text-left hover:underline"
										onclick={() => management.selectEmployee(employee.email)}
										aria-current={isSelected ? 'true' : undefined}
									>
										<PersonAvatar
											name={employee.displayName}
											email={employee.email}
											seed={employee.email || employee.displayName}
											class="size-8 shrink-0"
										/>
										<span class="grid">
											<span class="font-medium">{employee.displayName}</span>
											<span class="text-xs text-muted-foreground">{employee.email}</span>
										</span>
									</button>
								</Table.Cell>
								<Table.Cell class="text-right tabular-nums">
									{dayValue(employee.grantedMilliDays)}
								</Table.Cell>
								<Table.Cell class="text-right tabular-nums">
									{dayValue(employee.usedMilliDays)}
								</Table.Cell>
								<Table.Cell class="text-right tabular-nums">
									{dayValue(employee.reservedMilliDays)}
								</Table.Cell>
								<Table.Cell class="text-right font-medium tabular-nums">
									{dayValue(employee.availableMilliDays)}
								</Table.Cell>
								<Table.Cell class="text-right tabular-nums">
									{dayValue(employee.expiringMilliDays)}
								</Table.Cell>
							</Table.Row>
						{:else}
							<Table.Row>
								<Table.Cell colspan={6} class="h-28 text-center text-muted-foreground">
									{text.management.noEmployees}
								</Table.Cell>
							</Table.Row>
						{/each}
					</Table.Body>
				</Table.Root>
			</Card.Content>
		</Card.Root>

		<div class="min-w-0 space-y-5">
			{#if management.payload?.detail}
				{@const detail = management.payload.detail}
				<Card.Root>
					<Card.Header
						class="flex-row items-center gap-3"
						data-testid="leave-management-employee-detail-header"
					>
						<PersonAvatar
							name={detail.employee.displayName}
							email={detail.employee.email}
							seed={detail.employee.email || detail.employee.displayName}
							class="size-10 shrink-0"
						/>
						<div class="min-w-0">
							<Card.Title>{detail.employee.displayName}</Card.Title>
							<Card.Description>{detail.employee.email}</Card.Description>
						</div>
					</Card.Header>
					<Card.Content class="grid gap-3 sm:grid-cols-2">
						{#each detail.employee.balances as balance (balance.leaveTypeID)}
							<div class="rounded-lg border p-4">
								<div class="flex items-center justify-between gap-2">
									<p class="font-medium">
										{leaveTypeName(balance.leaveTypeID, balance.leaveTypeName)}
									</p>
									<Badge variant="secondary">
										{text.management.available} {dayValue(balance.availableMilliDays)}
									</Badge>
								</div>
								<div class="mt-3 grid grid-cols-3 gap-2 text-xs text-muted-foreground">
									<span>{text.management.granted} {dayValue(balance.grantedMilliDays)}</span>
									<span>{text.management.used} {dayValue(balance.usedMilliDays)}</span>
									<span>{text.management.pending} {dayValue(balance.reservedMilliDays)}</span>
								</div>
								{#if balance.nextExpiryDate}
									<p class="mt-2 text-xs text-muted-foreground">
										{text.management.expiryTemplate
											.replace('{date}', balance.nextExpiryDate)
											.replace('{amount}', dayValue(balance.nextExpiryMilliDays))}
									</p>
								{/if}
							</div>
						{/each}
					</Card.Content>
				</Card.Root>

				<Card.Root>
					<Card.Header>
						<Card.Title>{text.management.ledgerTitle}</Card.Title>
						<Card.Description>{text.management.ledgerDescription}</Card.Description>
					</Card.Header>
					<Card.Content class="space-y-3">
						{#each detail.ledgerEntries as entry (entry.id)}
							<div class="flex flex-wrap items-start justify-between gap-3 border-b pb-3 last:border-0 last:pb-0">
								<div class="min-w-0">
									<div class="flex flex-wrap items-center gap-2">
										<p class="font-medium">
											{leaveTypeName(entry.leaveTypeID, entry.leaveTypeName)}
										</p>
										<Badge variant="outline">{operationLabel(entry.operationType)}</Badge>
									</div>
									<p class="mt-1 text-xs text-muted-foreground">
										{entry.effectiveOn}
										{#if entry.reason} · {entry.reason}{/if}
									</p>
								</div>
								<div class="text-right tabular-nums">
									<p class={entry.deltaMilliDays < 0 ? 'text-destructive' : 'text-foreground'}>
										{signedDayValue(entry.deltaMilliDays)}
									</p>
									<p class="text-xs text-muted-foreground">
										{text.management.balanceAfter} {dayValue(entry.balanceAfterMilliDays)}
									</p>
								</div>
							</div>
						{:else}
							<p class="py-8 text-center text-sm text-muted-foreground">
								{text.management.noLedger}
							</p>
						{/each}
					</Card.Content>
				</Card.Root>

				<Card.Root>
					<Card.Header>
						<Card.Title>{text.management.requestHistory}</Card.Title>
					</Card.Header>
					<Card.Content class="space-y-3">
						{#each detail.requests as request (request.id)}
							<div
								class="flex flex-wrap items-start justify-between gap-3 border-b pb-3 last:border-0 last:pb-0"
								data-managed-leave-request={request.id}
							>
								<div>
									<div class="flex items-center gap-2">
										<p class="font-medium">
											{leaveTypeName(request.leaveTypeID, request.leaveTypeName)}
										</p>
										<Badge variant="secondary">{requestStatus(request.status)}</Badge>
									</div>
									<p class="mt-1 text-xs text-muted-foreground">
										{request.startDate}{request.endDate ? ` – ${request.endDate}` : ''}
										{#if request.startTime} · {request.startTime}{request.endTime ? `–${request.endTime}` : ''}{/if}
									</p>
								</div>
								<div class="flex items-center gap-2">
									<p class="font-medium tabular-nums">{dayValue(request.deductionMilliDays)}</p>
									{#if request.status === 'approved' && request.unit !== 'fullDay'}
										<LeaveTimeCorrectionDialog {request} />
									{/if}
									{#if request.status === 'approved'}
										<AlertDialog.Root>
											<AlertDialog.Trigger
												class={buttonVariants({ variant: 'outline', size: 'sm' })}
												disabled={management.isMutating}
											>
												{text.management.cancelApprovedAction}
											</AlertDialog.Trigger>
											<AlertDialog.Content data-testid="leave-management-cancel-dialog">
												<AlertDialog.Header>
													<AlertDialog.Title>
														{text.management.cancelApprovedTitle}
													</AlertDialog.Title>
													<AlertDialog.Description>
														{text.management.cancelApprovedDescription}
													</AlertDialog.Description>
												</AlertDialog.Header>
												<AlertDialog.Footer>
													<AlertDialog.Cancel>{text.cancel}</AlertDialog.Cancel>
													<AlertDialog.Action
														onclick={() => void management.cancelRequest(request.id)}
													>
														{text.management.cancelApprovedConfirmAction}
													</AlertDialog.Action>
												</AlertDialog.Footer>
											</AlertDialog.Content>
										</AlertDialog.Root>
									{/if}
								</div>
							</div>
						{:else}
							<p class="py-8 text-center text-sm text-muted-foreground">
								{text.management.noRequests}
							</p>
						{/each}
					</Card.Content>
				</Card.Root>
			{:else}
				<div class="grid min-h-72 place-items-center rounded-xl border border-dashed text-sm text-muted-foreground">
					{text.management.selectEmployeePrompt}
				</div>
			{/if}
		</div>
	</div>
</section>
