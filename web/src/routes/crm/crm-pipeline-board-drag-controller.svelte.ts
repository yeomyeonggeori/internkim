import { feelHaptic } from '$lib/native-shell/haptics';
import type { CRMPipelineBoardMoveRequest } from './crm-pipeline-board-drag';
import type { CRMOpportunity, CRMOpportunityStage } from './crm-types';

type CRMPipelineBoardDragControllerInput = {
	moveOpportunity: (request: CRMPipelineBoardMoveRequest) => void;
};

const boardDragDataType = 'application/x-internkim-crm-opportunity-id';

export class CRMPipelineBoardDragController {
	dropTarget = $state<CRMPipelineBoardMoveRequest | null>(null);

	private draggedOpportunityID = $state('');
	private moveOpportunity: (request: CRMPipelineBoardMoveRequest) => void = () => {};

	sync = (input: CRMPipelineBoardDragControllerInput): void => {
		this.moveOpportunity = input.moveOpportunity;
	};

	handleOpportunityDragStart = (event: DragEvent, opportunity: CRMOpportunity): void => {
		this.draggedOpportunityID = opportunity.id;
		feelHaptic('touch');
		event.dataTransfer?.setData(boardDragDataType, opportunity.id);
		event.dataTransfer?.setData('text/plain', opportunity.id);
		if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
	};

	handleOpportunityDragEnd = (): void => {
		this.draggedOpportunityID = '';
		this.dropTarget = null;
	};

	handleColumnDragOver = (
		event: DragEvent,
		stage: CRMOpportunityStage,
		columnOpportunities: CRMOpportunity[]
	): void => {
		const opportunityID = this.currentDragOpportunityID(event);
		if (!opportunityID) return;
		if (this.isSameColumnDropNoOp(opportunityID, null, columnOpportunities)) {
			this.dropTarget = null;
			return;
		}
		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
		this.dropTarget = { opportunityID, targetStage: stage, beforeOpportunityID: null };
	};

	handleColumnDrop = (
		event: DragEvent,
		stage: CRMOpportunityStage,
		columnOpportunities: CRMOpportunity[]
	): void => {
		event.preventDefault();
		const opportunityID = this.currentDragOpportunityID(event);
		if (!opportunityID) return;
		if (this.isSameColumnDropNoOp(opportunityID, null, columnOpportunities)) {
			this.handleOpportunityDragEnd();
			return;
		}
		feelHaptic('touch');
		this.moveOpportunity({ opportunityID, targetStage: stage, beforeOpportunityID: null });
		this.handleOpportunityDragEnd();
	};

	handleCardDragOver = (
		event: DragEvent,
		stage: CRMOpportunityStage,
		columnOpportunities: CRMOpportunity[],
		opportunity: CRMOpportunity
	): void => {
		const nextDropTarget = this.cardDropTarget(event, stage, columnOpportunities, opportunity);
		if (!nextDropTarget) {
			if (this.currentDragOpportunityID(event)) {
				event.stopPropagation();
				this.dropTarget = null;
			}
			return;
		}
		event.preventDefault();
		event.stopPropagation();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
		this.dropTarget = nextDropTarget;
	};

	handleCardDrop = (
		event: DragEvent,
		stage: CRMOpportunityStage,
		columnOpportunities: CRMOpportunity[],
		opportunity: CRMOpportunity
	): void => {
		const nextDropTarget = this.cardDropTarget(event, stage, columnOpportunities, opportunity);
		if (!nextDropTarget) {
			if (this.currentDragOpportunityID(event)) {
				event.preventDefault();
				event.stopPropagation();
				this.handleOpportunityDragEnd();
			}
			return;
		}
		event.preventDefault();
		event.stopPropagation();
		feelHaptic('touch');
		this.moveOpportunity(nextDropTarget);
		this.handleOpportunityDragEnd();
	};

	shouldShowCardInsertionLine = (stage: CRMOpportunityStage, opportunityID: string): boolean =>
		this.dropTarget?.targetStage === stage && this.dropTarget.beforeOpportunityID === opportunityID;

	shouldShowAppendInsertionLine = (stage: CRMOpportunityStage): boolean =>
		this.dropTarget?.targetStage === stage && this.dropTarget.beforeOpportunityID === null;

	private cardDropTarget(
		event: DragEvent,
		stage: CRMOpportunityStage,
		columnOpportunities: CRMOpportunity[],
		opportunity: CRMOpportunity
	): CRMPipelineBoardMoveRequest | null {
		const opportunityID = this.currentDragOpportunityID(event);
		if (!opportunityID || opportunityID === opportunity.id) return null;
		const currentTarget = event.currentTarget;
		if (!(currentTarget instanceof HTMLElement)) return null;
		const bounds = currentTarget.getBoundingClientRect();
		const isAfterOpportunity = event.clientY > bounds.top + bounds.height / 2;
		const opportunityIndex = columnOpportunities.findIndex((candidate) => candidate.id === opportunity.id);
		if (opportunityIndex < 0) return null;
		const nextOpportunity = isAfterOpportunity ? columnOpportunities[opportunityIndex + 1] : opportunity;
		const beforeOpportunityID = nextOpportunity?.id ?? null;
		if (this.isSameColumnDropNoOp(opportunityID, beforeOpportunityID, columnOpportunities)) return null;
		return { opportunityID, targetStage: stage, beforeOpportunityID };
	}

	private currentDragOpportunityID(event: DragEvent): string {
		return this.draggedOpportunityID || event.dataTransfer?.getData(boardDragDataType) || '';
	}

	private isSameColumnDropNoOp(
		opportunityID: string,
		beforeOpportunityID: string | null,
		columnOpportunities: CRMOpportunity[]
	): boolean {
		const draggedOpportunityIndex = columnOpportunities.findIndex((opportunity) => opportunity.id === opportunityID);
		if (draggedOpportunityIndex < 0) return false;
		if (beforeOpportunityID === null) return draggedOpportunityIndex === columnOpportunities.length - 1;
		const beforeOpportunityIndex = columnOpportunities.findIndex(
			(opportunity) => opportunity.id === beforeOpportunityID
		);
		return beforeOpportunityIndex === draggedOpportunityIndex
			|| beforeOpportunityIndex === draggedOpportunityIndex + 1;
	}
}
