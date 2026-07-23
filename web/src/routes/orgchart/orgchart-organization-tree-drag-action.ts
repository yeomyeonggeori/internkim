export type OrganizationTreeDragOptions = {
	groupID: string;
	onStart: (groupID: string, clientX: number, clientY: number) => void;
	onMove: (clientX: number, clientY: number) => void;
	onEnd: () => void;
	onCancel: () => void;
};

export function organizationTreeDrag(node: HTMLElement, initialOptions: OrganizationTreeDragOptions): { update: (options: OrganizationTreeDragOptions) => void; destroy: () => void } {
	let options = initialOptions;
	let activePointerID: number | undefined;

	const handlePointerDown = (event: PointerEvent): void => {
		if (event.button !== 0) return;
		event.preventDefault();
		activePointerID = event.pointerId;
		node.setPointerCapture(event.pointerId);
		options.onStart(options.groupID, event.clientX, event.clientY);
	};
	const handlePointerMove = (event: PointerEvent): void => {
		if (activePointerID !== event.pointerId) return;
		options.onMove(event.clientX, event.clientY);
	};
	const handlePointerUp = (event: PointerEvent): void => {
		if (activePointerID !== event.pointerId) return;
		activePointerID = undefined;
		options.onEnd();
	};
	const handlePointerCancel = (event: PointerEvent): void => {
		if (activePointerID !== event.pointerId) return;
		activePointerID = undefined;
		options.onCancel();
	};

	node.addEventListener('pointerdown', handlePointerDown);
	node.addEventListener('pointermove', handlePointerMove);
	node.addEventListener('pointerup', handlePointerUp);
	node.addEventListener('pointercancel', handlePointerCancel);

	return {
		update(nextOptions) {
			options = nextOptions;
		},
		destroy() {
			node.removeEventListener('pointerdown', handlePointerDown);
			node.removeEventListener('pointermove', handlePointerMove);
			node.removeEventListener('pointerup', handlePointerUp);
			node.removeEventListener('pointercancel', handlePointerCancel);
		}
	};
}
