import { useEffect, useId, useState } from 'react';

const lightTheme = {
  background: 'transparent',
  primaryColor: '#f4f4f5',
  primaryTextColor: '#18181b',
  primaryBorderColor: '#d4d4d8',
  lineColor: '#a1a1aa',
  secondaryColor: '#fafafa',
  tertiaryColor: '#fafafa',
  clusterBkg: 'transparent',
  clusterBorder: '#e4e4e7',
  edgeLabelBackground: '#ffffff',
};

const darkTheme = {
  background: 'transparent',
  primaryColor: '#27272a',
  primaryTextColor: '#fafafa',
  primaryBorderColor: '#52525b',
  lineColor: '#71717a',
  secondaryColor: '#18181b',
  tertiaryColor: '#18181b',
  clusterBkg: 'transparent',
  clusterBorder: '#3f3f46',
  edgeLabelBackground: '#18181b',
};

function isDarkDocument(): boolean {
  return document.documentElement.classList.contains('dark');
}

export function Mermaid({ chart }: { chart: string }) {
  const [renderedSVG, setRenderedSVG] = useState('');
  const identifier = useId().replace(/:/g, '');
  const [isDark, setIsDark] = useState(false);

  useEffect(() => {
    const observer = new MutationObserver(() => setIsDark(isDarkDocument()));
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ['class'] });
    setIsDark(isDarkDocument());
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    let isCurrent = true;

    async function render() {
      const { default: mermaid } = await import('mermaid');
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: 'strict',
        theme: 'base',
        themeVariables: isDark ? darkTheme : lightTheme,
        fontFamily: 'inherit',
        fontSize: 13,
      });
      const { svg } = await mermaid.render(`mermaid-${identifier}-${isDark}`, chart.trim());
      if (isCurrent) setRenderedSVG(svg);
    }

    render();
    return () => {
      isCurrent = false;
    };
  }, [chart, identifier, isDark]);

  return (
    <div
      className="my-6 overflow-x-auto rounded-lg border bg-fd-card p-4 [&_svg]:mx-auto"
      dangerouslySetInnerHTML={{ __html: renderedSVG }}
    />
  );
}
