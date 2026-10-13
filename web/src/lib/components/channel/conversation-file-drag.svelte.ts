import type { Action } from 'svelte/action';

type ConversationFileDragOptions = {
	onFiles: (files: File[]) => void;
	isDisabled: () => boolean;
};

function carriesFiles(event: DragEvent): boolean {
	return event.dataTransfer?.types.includes('Files') ?? false;
}

export class ConversationFileDrag {
	isOver = $state(false);
	#depth = 0;

	constructor(private readonly options: ConversationFileDragOptions) {}

	#accepts(event: DragEvent): boolean {
		return carriesFiles(event) && !this.options.isDisabled();
	}

	enter(event: DragEvent): void {
		if (!this.#accepts(event)) return;
		event.preventDefault();
		this.#depth += 1;
		this.isOver = true;
	}

	over(event: DragEvent): void {
		if (!this.#accepts(event)) return;
		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'copy';
	}

	leave(event: DragEvent): void {
		if (!carriesFiles(event)) return;
		this.#depth = Math.max(0, this.#depth - 1);
		if (this.#depth === 0) this.isOver = false;
	}

	drop(event: DragEvent): void {
		const accepted = this.#accepts(event);
		this.#depth = 0;
		this.isOver = false;
		if (!accepted) return;
		event.preventDefault();
		const files = Array.from(event.dataTransfer?.files ?? []);
		if (files.length > 0) this.options.onFiles(files);
	}
}

export const receivesFileDrag: Action<HTMLElement, ConversationFileDrag> = (node, drag) => {
	const enter = (event: DragEvent) => drag.enter(event);
	const over = (event: DragEvent) => drag.over(event);
	const leave = (event: DragEvent) => drag.leave(event);
	const drop = (event: DragEvent) => drag.drop(event);
	node.addEventListener('dragenter', enter);
	node.addEventListener('dragover', over);
	node.addEventListener('dragleave', leave);
	node.addEventListener('drop', drop);
	return {
		destroy() {
			node.removeEventListener('dragenter', enter);
			node.removeEventListener('dragover', over);
			node.removeEventListener('dragleave', leave);
			node.removeEventListener('drop', drop);
		}
	};
};
