export const standInRateDate = '2026-10-06';

export const standInCurrencies: Record<string, { name: string; unitsPerDollar: number }> = {
	USD: { name: 'United States Dollar', unitsPerDollar: 1 },
	KRW: { name: 'South Korean Won', unitsPerDollar: 1500 },
	EUR: { name: 'Euro', unitsPerDollar: 0.8 },
	GBP: { name: 'British Pound', unitsPerDollar: 0.75 },
	JPY: { name: 'Japanese Yen', unitsPerDollar: 150 }
};

function currenciesAnswer(): Response {
	return Response.json(Object.entries(standInCurrencies).map(([isoCode, { name }]) => ({ iso_code: isoCode, name })));
}

function ratesAnswer(url: URL): Response {
	const base = url.searchParams.get('base') ?? '';
	const quotes = (url.searchParams.get('quotes') ?? '').split(',');
	const unknown = [base, ...quotes].filter((code) => standInCurrencies[code] === undefined);
	if (unknown.length > 0) return Response.json({ message: `not found: ${unknown.join(', ')}` }, { status: 404 });
	return Response.json(
		quotes.map((quote) => ({
			date: standInRateDate,
			base,
			quote,
			rate: standInCurrencies[quote].unitsPerDollar / standInCurrencies[base].unitsPerDollar
		}))
	);
}

export function answerAsFrankfurter(request: Request): Response {
	const url = new URL(request.url);
	if (url.pathname === '/v2/currencies') return currenciesAnswer();
	if (url.pathname === '/v2/rates') return ratesAnswer(url);
	return new Response('not found', { status: 404 });
}

if (import.meta.main) {
	Bun.serve({ hostname: '127.0.0.1', port: Number(process.argv[2]), fetch: answerAsFrankfurter });
}
