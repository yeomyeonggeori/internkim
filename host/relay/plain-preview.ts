import { lexer, type MarkedToken, type Token, type Tokens } from 'marked';

const cellSeparator = ' · ';
const unorderedMarker = '•';

const markedTokenTypes = {
	blockquote: true,
	br: true,
	checkbox: true,
	code: true,
	codespan: true,
	def: true,
	del: true,
	em: true,
	escape: true,
	heading: true,
	hr: true,
	html: true,
	image: true,
	link: true,
	list: true,
	list_item: true,
	paragraph: true,
	space: true,
	strong: true,
	table: true,
	text: true
} satisfies Record<MarkedToken['type'], true>;

export function plainPreviewOf(message: string): string {
	return withoutBlankLines(blockTextOf(lexer(message))).trim();
}

function withoutBlankLines(text: string): string {
	return text
		.split('\n')
		.filter((line) => line.trim() !== '')
		.join('\n');
}

function isMarkedToken(token: Token): token is MarkedToken {
	return Object.hasOwn(markedTokenTypes, token.type);
}

function blockTextOf(tokens: Token[]): string {
	return tokens.filter(isMarkedToken).map(plainTextOf).join('\n');
}

function inlineTextOf(tokens: Token[]): string {
	return tokens.filter(isMarkedToken).map(plainTextOf).join('');
}

function plainTextOf(token: MarkedToken): string {
	switch (token.type) {
		case 'space':
		case 'hr':
		case 'def':
		case 'checkbox':
			return '';
		case 'br':
			return '\n';
		case 'code':
		case 'codespan':
		case 'escape':
		case 'html':
			return token.text;
		case 'text':
			return token.tokens ? inlineTextOf(token.tokens) : token.text;
		case 'strong':
		case 'em':
		case 'del':
		case 'link':
		case 'image':
		case 'heading':
		case 'paragraph':
			return inlineTextOf(token.tokens);
		case 'blockquote':
		case 'list_item':
			return blockTextOf(token.tokens);
		case 'list':
			return listTextOf(token);
		case 'table':
			return tableTextOf(token);
	}
}

function listTextOf(list: Tokens.List): string {
	return list.items.map((item, index) => itemTextOf(item, markerOf(list, index))).join('\n');
}

function markerOf(list: Tokens.List, index: number): string {
	if (!list.ordered) return unorderedMarker;
	const start = typeof list.start === 'number' ? list.start : 1;
	return `${start + index}.`;
}

function itemTextOf(item: Tokens.ListItem, marker: string): string {
	const ownText = withoutBlankLines(blockTextOf(item.tokens.filter((token) => token.type !== 'list')));
	const nestedText = blockTextOf(item.tokens.filter((token) => token.type === 'list'));
	return [`${marker} ${ownText}`.trimEnd(), nestedText].join('\n');
}

function tableTextOf(table: Tokens.Table): string {
	return [table.header, ...table.rows].map(rowTextOf).join('\n');
}

function rowTextOf(cells: Tokens.TableCell[]): string {
	return cells.map((cell) => inlineTextOf(cell.tokens)).join(cellSeparator);
}
