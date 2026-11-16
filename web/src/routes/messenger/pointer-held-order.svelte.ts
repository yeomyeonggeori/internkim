type Identified = { id: string };

export class PointerHeldOrder<Row extends Identified> {
	#isHeld = $state(false);
	#heldIDs: string[] = [];
	readonly #live: () => Row[];

	readonly shown: Row[] = $derived.by(() => {
		const live = this.#live();
		if (!this.#isHeld) return live;
		const heldPosition = new Map(this.#heldIDs.map((id, index) => [id, index]));
		const positionOf = (row: Row) => heldPosition.get(row.id) ?? heldPosition.size;
		return live.toSorted((first, second) => positionOf(first) - positionOf(second));
	});

	constructor(live: () => Row[]) {
		this.#live = live;
	}

	hold = (): void => {
		this.#heldIDs = this.shown.map((row) => row.id);
		this.#isHeld = true;
	};

	release = (): void => {
		this.#isHeld = false;
	};
}
