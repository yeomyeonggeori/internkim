import {
  isRouteErrorResponse,
  Links,
  Meta,
  Outlet,
  Scripts,
  ScrollRestoration,
  useLocation,
  useNavigate,
} from 'react-router';
import { RootProvider } from 'fumadocs-ui/provider/react-router';
import { localeOfPath, pathInLocale } from '@/lib/i18n';
import { i18nUI } from '@/lib/layout.shared';
import type { Route } from './+types/root';
import './app.css';
import SearchDialog from '@/components/search';
import NotFound from './routes/not-found';

export const links: Route.LinksFunction = () => [
  { rel: 'icon', href: '/favicon.svg', type: 'image/svg+xml' },
  { rel: 'icon', href: '/favicon.png', type: 'image/png', sizes: '32x32' },
  { rel: 'icon', href: '/favicon-192.png', type: 'image/png', sizes: '192x192' },
  { rel: 'icon', href: '/favicon-512.png', type: 'image/png', sizes: '512x512' },
];

export function Layout({ children }: { children: React.ReactNode }) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const lang = localeOfPath(pathname);

  return (
    <html lang={lang} suppressHydrationWarning>
      <head>
        <meta charSet="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <Meta />
        <Links />
      </head>
      <body className="flex flex-col min-h-screen">
        <RootProvider
          search={{ SearchDialog }}
          i18n={{
            ...i18nUI.provider(lang),
            onLocaleChange: (locale) => navigate(pathInLocale(pathname, locale)),
          }}
        >
          {children}
        </RootProvider>
        <ScrollRestoration />
        <Scripts />
      </body>
    </html>
  );
}

export default function App() {
  return <Outlet />;
}

export function ErrorBoundary({ error }: Route.ErrorBoundaryProps) {
  let message = 'Oops!';
  let details = 'An unexpected error occurred.';
  let stack: string | undefined;

  if (isRouteErrorResponse(error)) {
    if (error.status === 404) return <NotFound />;
    message = 'Error';
    details = error.statusText;
  } else if (import.meta.env.DEV && error && error instanceof Error) {
    details = error.message;
    stack = error.stack;
  }

  return (
    <main className="pt-16 p-4 w-full max-w-[1400px] mx-auto">
      <h1>{message}</h1>
      <p>{details}</p>
      {stack && (
        <pre className="w-full p-4 overflow-x-auto">
          <code>{stack}</code>
        </pre>
      )}
    </main>
  );
}
