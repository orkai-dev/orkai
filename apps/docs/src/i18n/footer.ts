type FooterStrings = {
  brandTagline: string;
  brandMeta: string;
  builtOn: string;
};

const strings: Record<string, FooterStrings> = {
  en: {
    brandTagline: 'Self-hosted PaaS on K3s. Every workload is a real Kubernetes object.',
    brandMeta: 'open source · K3s-native',
    builtOn: 'built on Kubernetes',
  },
  es: {
    brandTagline:
      'PaaS autoalojado en K3s. Cada carga de trabajo es un objeto Kubernetes real.',
    brandMeta: 'código abierto · nativo en K3s',
    builtOn: 'construido sobre Kubernetes',
  },
  pt: {
    brandTagline:
      'PaaS self-hosted em K3s. Cada workload é um objeto Kubernetes real.',
    brandMeta: 'código aberto · nativo em K3s',
    builtOn: 'construído sobre Kubernetes',
  },
};

export function getFooterStrings(locale: string): FooterStrings {
  return strings[locale] ?? strings.en;
}
