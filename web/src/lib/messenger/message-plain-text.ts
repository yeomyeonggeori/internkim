import { marked, type Token } from 'marked';

const blockTypes = new Set(['paragraph', 'heading', 'blockquote', 'list', 'list_item', 'code', 'table', 'hr', 'space']);

export function messagePlainText(markdown: string): string {
	return plainTextOf(marked.lexer(markdown)).replace(/\s+/g, ' ').trim();
}

function plainTextOf(tokens: Token[]): string {
	return tokens.map((token) => (blockTypes.has(token.type) ? ` ${textOf(token)} ` : textOf(token))).join('');
}

function textOf(token: Token): string {
	if (token.type === 'html') return '';
	if (token.type === 'br') return ' ';
	if (token.type === 'list') return token.items.map((item: Token) => ` ${textOf(item)} `).join('');
	if ('tokens' in token && Array.isArray(token.tokens)) return plainTextOf(token.tokens);
	if ('text' in token && typeof token.text === 'string') return token.text;
	return '';
}
