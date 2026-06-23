export type NotFoundContent = {
  eyebrow: string;
  title: string;
  lead: string;
  terminalLabel: string;
  terminalOutput: string;
  ctaHome: string;
  ctaDocs: string;
};

const en: NotFoundContent = {
  eyebrow: '// 404 · not found',
  title: "This page isn't in the cluster.",
  lead: 'The URL may be wrong, moved, or never existed. Head back to the docs or the home page.',
  terminalLabel: 'kubectl',
  terminalOutput: 'Error from server (NotFound): pages "unknown" not found',
  ctaHome: 'Back to home',
  ctaDocs: 'Read the docs',
};

const es: NotFoundContent = {
  eyebrow: '// 404 · no encontrada',
  title: 'Esta página no está en el clúster.',
  lead:
    'La URL puede ser incorrecta, haberse movido o no existir. Vuelve a la documentación o a la página principal.',
  terminalLabel: 'kubectl',
  terminalOutput: 'Error from server (NotFound): pages "unknown" not found',
  ctaHome: 'Volver al inicio',
  ctaDocs: 'Leer la documentación',
};

const pt: NotFoundContent = {
  eyebrow: '// 404 · não encontrada',
  title: 'Esta página não está no cluster.',
  lead:
    'A URL pode estar errada, ter sido movida ou nunca ter existido. Volte para a documentação ou para a página inicial.',
  terminalLabel: 'kubectl',
  terminalOutput: 'Error from server (NotFound): pages "unknown" not found',
  ctaHome: 'Voltar ao início',
  ctaDocs: 'Ler a documentação',
};

const contentByLocale: Record<string, NotFoundContent> = {en, es, pt};

export function getNotFoundContent(locale: string): NotFoundContent {
  return contentByLocale[locale] ?? en;
}
