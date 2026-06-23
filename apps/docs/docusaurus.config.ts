import {orkaiDark, orkaiLight} from './src/theme/prism-orkai';
import type {Config} from '@docusaurus/types';
import type * as Preset from '@docusaurus/preset-classic';

const SITE_URL = 'https://orkai.dev';

const SEO = {
  title: "orka'i — Self-hosted PaaS, powered by Kubernetes",
  tagline: 'Self-hosted PaaS, powered by Kubernetes',
  description:
    'Deploy apps, databases, and cron jobs to your own servers with real Kubernetes under the hood. From a single node to a full cluster — no Docker wrappers, no cloud lock-in.',
  docsDescription:
    "Official orka'i documentation — installation, deployment guides, and API reference for your self-hosted K3s PaaS.",
  keywords:
    'PaaS, self-hosted, Kubernetes, K3s, open source, deploy, Heroku alternative, cloud platform, orkai documentation',
  ogDescription:
    'Deploy to your own servers with real Kubernetes. From one node to a full cluster — no Docker wrappers, no cloud lock-in.',
} as const;

const config: Config = {
  title: "orka'i",
  tagline: SEO.tagline,
  favicon: 'img/favicon.svg',

  future: {
    v4: true,
  },

  url: SITE_URL,
  baseUrl: '/',

  organizationName: 'orkai-dev',
  projectName: 'orkai',

  onBrokenLinks: 'throw',

  customFields: {
    seoDescription: SEO.docsDescription,
  },

  i18n: {
    defaultLocale: 'en',
    locales: ['en', 'es', 'pt'],
    localeConfigs: {
      en: {label: 'English', htmlLang: 'en', direction: 'ltr'},
      es: {label: 'Español', htmlLang: 'es', direction: 'ltr'},
      pt: {label: 'Português', htmlLang: 'pt', direction: 'ltr'},
    },
  },

  headTags: [
    {
      tagName: 'link',
      attributes: {rel: 'preconnect', href: 'https://fonts.googleapis.com'},
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'preconnect',
        href: 'https://fonts.gstatic.com',
        crossorigin: 'anonymous',
      },
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700&family=Geist+Mono:wght@400;500;700&family=Inter:wght@400;500;600;700&display=swap',
      },
    },
    {
      tagName: 'link',
      attributes: {
        rel: 'stylesheet',
        href: 'https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:opsz,wght,FILL@20..48,100..700,0..1&display=swap',
      },
    },
    {
      tagName: 'link',
      attributes: {rel: 'apple-touch-icon', href: '/img/favicon.svg'},
    },
    {
      tagName: 'link',
      attributes: {rel: 'manifest', href: '/site.webmanifest'},
    },
    {
      tagName: 'script',
      attributes: {type: 'application/ld+json'},
      innerHTML: JSON.stringify({
        '@context': 'https://schema.org',
        '@type': 'WebSite',
        name: "orka'i Documentation",
        url: SITE_URL,
        description: SEO.docsDescription,
        publisher: {
          '@type': 'Organization',
          name: "orka'i",
          url: SITE_URL,
          logo: {
            '@type': 'ImageObject',
            url: `${SITE_URL}/img/logo.svg`,
          },
        },
      }),
    },
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: './sidebars.ts',
          routeBasePath: 'docs',
        },
        blog: false,
        sitemap: {
          changefreq: 'weekly',
          priority: 0.5,
          filename: 'sitemap.xml',
        },
        theme: {
          customCss: './src/css/custom.css',
        },
      } satisfies Preset.Options,
    ],
  ],

  themeConfig: {
    image: 'img/og.png',
    colorMode: {
      defaultMode: 'light',
      disableSwitch: false,
      respectPrefersColorScheme: true,
    },
    metadata: [
      {name: 'description', content: SEO.docsDescription},
      {name: 'keywords', content: SEO.keywords},
      {name: 'author', content: "orka'i"},
      {name: 'theme-color', content: '#f8fbfc'},
      {property: 'og:type', content: 'website'},
      {property: 'og:site_name', content: "orka'i"},
      {property: 'og:title', content: SEO.title},
      {property: 'og:description', content: SEO.ogDescription},
      {name: 'twitter:card', content: 'summary_large_image'},
    ],
    navbar: {
      title: "orka'i",
      logo: {
        alt: "orka'i logo",
        src: 'img/logo.svg',
      },
      items: [
        {
          type: 'docSidebar',
          sidebarId: 'docsSidebar',
          position: 'left',
          label: 'Docs',
        },
        {
          type: 'localeDropdown',
          position: 'right',
        },
        {
          type: 'custom-githubLink',
          href: 'https://github.com/orkai-dev/orkai',
          label: 'GitHub',
          position: 'right',
        },
        {
          to: '/docs/getting-started/installation',
          label: 'Self-host',
          position: 'right',
          className: 'navbar-cta',
        },
      ],
    },
    footer: {
      style: 'light',
      links: [
        {
          title: 'Docs',
          items: [
            {label: 'Overview', to: '/docs/intro'},
            {label: 'Installation', to: '/docs/getting-started/installation'},
            {label: 'Quick Start', to: '/docs/getting-started/quick-start'},
          ],
        },
        {
          title: 'Community',
          items: [
            {
              label: 'GitHub',
              href: 'https://github.com/orkai-dev/orkai',
            },
            {
              label: 'Discussions',
              href: 'https://github.com/orkai-dev/orkai/discussions',
            },
            {
              label: 'Issues',
              href: 'https://github.com/orkai-dev/orkai/issues',
            },
          ],
        },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} orka'i.`,
    },
    prism: {
      theme: orkaiLight,
      darkTheme: orkaiDark,
      additionalLanguages: ['bash', 'yaml', 'go', 'docker'],
    },
  } satisfies Preset.ThemeConfig,
};

export default config;
