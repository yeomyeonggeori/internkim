const personReferenceForms = 'name or job title without honorific or particle, email, @handle, or the personID a tool result showed';

export function describePersonHint(subject: string): string {
  return `${subject}: ${personReferenceForms}.`;
}

export function describePersonHints(subject: string): string {
  return `${subject}, each by ${personReferenceForms}.`;
}
