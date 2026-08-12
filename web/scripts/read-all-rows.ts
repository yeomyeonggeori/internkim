type Page<Row> = { data: Row[] | null; error: { message: string } | null };

// PostgREST caps an unbounded select at db-max-rows, 1000 by default, silently.
const pageSize = 1000;

export async function readAllRows<Row>(
	page: (from: number, to: number) => PromiseLike<Page<Row>>
): Promise<Row[]> {
	const rows: Row[] = [];
	for (let from = 0; ; from += pageSize) {
		const { data, error } = await page(from, from + pageSize - 1);
		if (error) throw new Error(error.message);
		rows.push(...(data ?? []));
		if ((data?.length ?? 0) < pageSize) return rows;
	}
}
