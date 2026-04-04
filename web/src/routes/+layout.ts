const isBoard = import.meta.env.VITE_BUILD_TARGET === 'board';

export const prerender = isBoard;
export const ssr = !isBoard;
