import { describe, expect, test } from "bun:test";
import { mirrorPending, type BuzzMMPendingItem } from "../../src/lib/buzz-mm-sync";

function item(externalId: string, updatedAt: number): BuzzMMPendingItem {
	return { externalId, externalChannelId: "mm", buzzChannelId: "bc", text: externalId, updatedAt };
}

describe("mirrorPending", () => {
	test("mirrors all items and advances the cursor to the newest", async () => {
		const done: string[] = [];
		const result = await mirrorPending([item("a", 30), item("b", 10), item("c", 20)], 0, async (i) => {
			done.push(i.externalId);
		});
		expect(done).toEqual(["b", "c", "a"]);
		expect(result).toEqual({ cursor: 30, mirrored: 3 });
	});

	test("stops at the first failure so later items are retried next time", async () => {
		const done: string[] = [];
		const result = await mirrorPending([item("a", 10), item("b", 20), item("c", 30)], 5, async (i) => {
			if (i.externalId === "b") throw new Error("relay down");
			done.push(i.externalId);
		});
		expect(done).toEqual(["a"]);
		expect(result).toEqual({ cursor: 10, mirrored: 1 });
	});

	test("does not advance the cursor when the first item fails", async () => {
		const result = await mirrorPending([item("a", 10)], 5, async () => {
			throw new Error("fail");
		});
		expect(result).toEqual({ cursor: 5, mirrored: 0 });
	});

	test("keeps the cursor when there is nothing pending", async () => {
		const result = await mirrorPending([], 42, async () => {});
		expect(result).toEqual({ cursor: 42, mirrored: 0 });
	});
});
