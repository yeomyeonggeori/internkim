<script lang="ts">
	import { currentLocale } from '$lib/i18n/locale.svelte';
	import { Badge } from '$lib/components/ui/badge';
	import type { CRMOrganization, CRMNextAction, CRMOpportunity, CRMPipelineStage } from './crm-types';
	import { CRMPipelineBoardDragController } from './crm-pipeline-board-drag-controller.svelte';
	import { crmLabel } from './crm-labels';
	import type { CRMPipelineBoardMoveRequest } from './crm-pipeline-board-drag';
	import { findOrganizationByID, findNextActionByID, formatCRMDate, getProgressKind, opportunityStageLabel } from './crm-view-model';
	import { formatViewMoney } from './crm-money';
	import { crmViewCurrency } from './crm-view-currency.svelte';
	import type { CurrencyCatalogue } from '$lib/currency/currency-catalogue';
	import type { CRMText } from './text';

	type Props = {
		opportunities: CRMOpportunity[];
		organizations: CRMOrganization[];
		nextActions: CRMNextAction[];
		stages: CRMPipelineStage[];
		currencyCatalogue: CurrencyCatalogue;
		text: CRMText;
		onMove: (request: CRMPipelineBoardMoveRequest) => void;
	};

	let { opportunities, organizations, nextActions, stages, currencyCatalogue, text, onMove }: Props = $props();

	const columnClass = [
		'crm-pipeline-board-column group flex h-full min-h-0',
		'shrink-0 snap-start flex-col overflow-hidden rounded-lg border bg-muted/30'
	].join(' ');
	const boardScrollClass = [
		'h-[var(--crm-pipeline-board-height,32rem)] min-h-80 min-w-0',
		'overflow-x-auto overflow-y-hidden px-4 pb-2 scroll-px-4 sm:px-6 sm:scroll-px-6 lg:px-8 lg:scroll-px-8',
		'snap-x snap-mandatory'
	].join(' ');
	const cardClass = [
		'grid cursor-grab gap-2 rounded-md border border-border/80 bg-card p-3 shadow-xs',
		'transition-[background-color,box-shadow,opacity] hover:bg-muted/30 hover:shadow-sm',
		'active:cursor-grabbing active:bg-muted/40'
	].join(' ');
	const insertionLineWrapperClass = 'flex h-4 items-center px-1';
	const insertionLineClass = 'h-0.5 w-full rounded-full bg-primary shadow-sm ring-1 ring-primary/20';
	const boardDrag = new CRMPipelineBoardDragController();

	$effect(() => {
		boardDrag.sync({ moveOpportunity: onMove });
	});

	function insertionIndicatorID(stage: string, beforeOpportunityID: string): string {
		return `${stage}:${beforeOpportunityID}`;
	}
</script>

<div class="sticky top-0 -mx-4 min-w-0 bg-background sm:-mx-6 lg:-mx-8">
	<div
		class={boardScrollClass}
		data-crm-pipeline-board-scroll
	>
		<div class="flex h-full min-w-max gap-3">
			{#each stages as stageDefinition (stageDefinition.stage)}
					{@const stage = stageDefinition.stage}
					{@const canDrag = stageDefinition.outcome !== 'won' && stageDefinition.outcome !== 'lost'}
					{@const stageOpportunities = opportunities.filter((opportunity) => opportunity.stage === stage)}
				<section
					class={columnClass}
					role="group"
					aria-label={opportunityStageLabel(stages, stage, text)}
					data-crm-pipeline-column={stage}
					ondragover={(event) => boardDrag.handleColumnDragOver(event, stage, stageOpportunities)}
					ondrop={(event) => boardDrag.handleColumnDrop(event, stage, stageOpportunities)}
				>
					<header class="flex h-11 items-center justify-between gap-3 border-b bg-card px-3">
						<h2 class="truncate text-sm font-semibold text-foreground">{opportunityStageLabel(stages, stage, text)}</h2>
						<span
							class="inline-flex h-5 min-w-5 shrink-0 items-center justify-center rounded-full bg-muted px-1.5 text-xs font-medium tabular-nums text-muted-foreground"
							aria-label={`${opportunityStageLabel(stages, stage, text)} ${stageOpportunities.length}`}
							data-crm-pipeline-count
						>
							{stageOpportunities.length}
						</span>
					</header>

					<div
						class="min-h-0 flex-1 overflow-y-auto px-2.5 pb-3 pt-3"
						role="list"
						aria-label={opportunityStageLabel(stages, stage, text)}
						ondragover={(event) => boardDrag.handleColumnDragOver(event, stage, stageOpportunities)}
						ondrop={(event) => boardDrag.handleColumnDrop(event, stage, stageOpportunities)}
					>
						<div class="space-y-2">
							{#each stageOpportunities as opportunity (opportunity.id)}
								{#if boardDrag.shouldShowCardInsertionLine(stage, opportunity.id)}
									<div
										class={insertionLineWrapperClass}
										data-crm-pipeline-drop-indicator={insertionIndicatorID(stage, opportunity.id)}
									>
										<div class={insertionLineClass}></div>
									</div>
								{/if}

								{@const organization = findOrganizationByID(organizations, opportunity.organizationID)}
								{@const action = findNextActionByID(nextActions, opportunity.nextActionID)}
									<article
										class={`${cardClass} ${canDrag ? '' : 'cursor-default active:cursor-default'}`}
										role="listitem"
										draggable={canDrag}
									data-crm-opportunity-card={opportunity.id}
									ondragstart={(event) => boardDrag.handleOpportunityDragStart(event, opportunity)}
									ondragend={boardDrag.handleOpportunityDragEnd}
									ondragover={(event) => boardDrag.handleCardDragOver(event, stage, stageOpportunities, opportunity)}
									ondrop={(event) => boardDrag.handleCardDrop(event, stage, stageOpportunities, opportunity)}
								>
									<div class="min-w-0">
										<p class="line-clamp-2 text-sm font-semibold leading-5 text-card-foreground">{opportunity.name}</p>
										<p class="mt-1 truncate text-xs text-muted-foreground">{organization?.name ?? text.none}</p>
									</div>
									<div class="flex flex-wrap gap-1.5">
										<Badge variant="outline" class="h-5 rounded-md border-border/70 bg-muted/30 px-1.5 py-0 text-[11px] font-normal text-muted-foreground shadow-none">
											{crmLabel(text.progressKinds, getProgressKind(opportunity, organization))}
										</Badge>
										<Badge variant="secondary" class="h-5 rounded-md bg-muted px-1.5 py-0 text-[11px] font-medium text-foreground/75 shadow-none">
											{opportunity.expectedValue === undefined ? text.noValue : formatViewMoney(crmViewCurrency.viewAmount(opportunity.expectedValue, opportunity.currency), currencyCatalogue, text.noValue, currentLocale.value, crmViewCurrency.selected)}
										</Badge>
									</div>
									<div class="text-xs leading-5 text-muted-foreground">
										<p class="line-clamp-2">{action?.title ?? text.none}</p>
										<p class="mt-0.5">{text.targetDate} {formatCRMDate(opportunity.targetDate, currentLocale.value)}</p>
									</div>
								</article>
							{:else}
								<p class="px-2 py-8 text-center text-xs leading-5 text-muted-foreground">{text.noProgress}</p>
							{/each}

							<div
								class="min-h-8"
								role="listitem"
								data-crm-pipeline-drop-zone={stage}
								ondragover={(event) => boardDrag.handleColumnDragOver(event, stage, stageOpportunities)}
								ondrop={(event) => boardDrag.handleColumnDrop(event, stage, stageOpportunities)}
							>
								{#if boardDrag.shouldShowAppendInsertionLine(stage)}
									<div
										class={insertionLineWrapperClass}
										data-crm-pipeline-drop-indicator={insertionIndicatorID(stage, 'append')}
									>
										<div class={insertionLineClass}></div>
									</div>
								{/if}
							</div>
						</div>
					</div>
				</section>
			{/each}
		</div>
	</div>
</div>

<style>
	[data-crm-pipeline-board-scroll] {
		container-type: inline-size;
		scrollbar-width: none;
	}

	[data-crm-pipeline-board-scroll]::-webkit-scrollbar {
		display: none;
	}

	.crm-pipeline-board-column {
		width: max(13.5rem, calc((100cqw + 1.25rem) / 1.5));
	}

	@container (min-width: 560px) {
		.crm-pipeline-board-column {
			width: max(13.5rem, calc((100cqw + 0.5rem) / 2.5));
		}
	}

	@container (min-width: 820px) {
		.crm-pipeline-board-column {
			width: max(15rem, calc((100cqw - 0.25rem) / 3.5));
		}
	}

	@container (min-width: 1180px) {
		.crm-pipeline-board-column {
			width: max(15.5rem, calc((100cqw - 1rem) / 4.5));
		}
	}
</style>
