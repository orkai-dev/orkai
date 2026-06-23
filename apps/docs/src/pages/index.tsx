import type {ReactNode} from 'react';
import {useCallback, useEffect, useRef, useState} from 'react';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';

import {getHomepageContent, type HomepageContent} from '../i18n/homepage';
import styles from './index.module.css';

const GITHUB_URL = 'https://github.com/orkai-dev/orkai';
const INSTALL_URL = '/docs/getting-started/installation';
const DOCS_URL = '/docs/intro';
const INSTALL_CMD = 'curl -sfL https://get.orkai.io | sh -';
const IMPL_PAGES_URL = 'https://github.com/orkai-dev/orkai/blob/main/.spec/impl/pages.md';

function MaterialIcon({name, className = ''}: {name: string; className?: string}) {
  return (
    <span
      aria-hidden="true"
      className={`material-symbols-outlined ${className}`}
      style={{fontVariationSettings: "'FILL' 1, 'wght' 400"}}>
      {name}
    </span>
  );
}

function useOnScreen(options?: IntersectionObserverInit) {
  const ref = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return undefined;

    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) {
        setVisible(true);
        observer.disconnect();
      }
    }, options ?? {threshold: 0.12, rootMargin: '0px 0px -60px 0px'});

    observer.observe(el);
    return () => observer.disconnect();
  }, [options]);

  return {ref, visible};
}

function FadeIn({
  children,
  className = '',
  delay = 0,
}: {
  children: ReactNode;
  className?: string;
  delay?: number;
}) {
  const {ref, visible} = useOnScreen();
  return (
    <div
      ref={ref}
      className={`${styles.fadeIn} ${className}`}
      data-visible={visible}
      style={{transitionDelay: `${delay}ms`}}>
      {children}
    </div>
  );
}

function prefersReducedMotion() {
  return (
    typeof window !== 'undefined' &&
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches === true
  );
}

type TerminalToken = {text: string; cls?: string};
type TerminalLine = TerminalToken[];

/** Reveals terminal lines one-by-one once the block scrolls into view. */
function useLineReveal(count: number, step = 380) {
  const {ref, visible} = useOnScreen();
  const [shown, setShown] = useState(0);

  useEffect(() => {
    if (!visible) return undefined;
    if (prefersReducedMotion()) {
      setShown(count);
      return undefined;
    }

    setShown(0);
    let i = 0;
    const id = window.setInterval(() => {
      i += 1;
      setShown(i);
      if (i >= count) window.clearInterval(id);
    }, step);
    return () => window.clearInterval(id);
  }, [visible, count, step]);

  return {ref, shown};
}

function AnimatedTerminal({
  label,
  lines,
  rightSlot,
  step,
}: {
  label: string;
  lines: TerminalLine[];
  rightSlot?: ReactNode;
  step?: number;
}) {
  const {ref, shown} = useLineReveal(lines.length, step);

  return (
    <div ref={ref} className={styles.terminal}>
      <div className={styles.terminalHeader}>
        <span className={styles.terminalLabel}>{label}</span>
        {rightSlot}
      </div>
      <pre className={styles.terminalBody}>
        <code>
          {lines.map((line, i) => {
            const isShown = i < shown;
            const isCaretLine = i === shown - 1;
            return (
              <span
                // eslint-disable-next-line react/no-array-index-key
                key={i}
                className={styles.termLine}
                data-shown={isShown}>
                {line.map((tok, j) => (
                  // eslint-disable-next-line react/no-array-index-key
                  <span key={j} className={tok.cls ? styles[tok.cls] : undefined}>
                    {tok.text}
                  </span>
                ))}
                {isCaretLine && <span className={styles.caret} aria-hidden="true" />}
                {'\n'}
              </span>
            );
          })}
        </code>
      </pre>
    </div>
  );
}

function CopyButton({
  value,
  label,
  copiedLabel,
  copyLabel,
}: {
  value: string;
  label: string;
  copiedLabel: string;
  copyLabel: string;
}) {
  const [copied, setCopied] = useState(false);

  const onCopy = useCallback(() => {
    navigator.clipboard?.writeText(value).then(() => {
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    });
  }, [value]);

  return (
    <button
      type="button"
      className={styles.copyButton}
      onClick={onCopy}
      aria-label={copied ? copiedLabel : `${copyLabel} ${label}`}>
      <MaterialIcon name={copied ? 'check' : 'content_copy'} className={styles.copyIcon} />
      <span>{copied ? copiedLabel : copyLabel}</span>
    </button>
  );
}

function InstallTerminal({t}: {t: HomepageContent}) {
  const lines: TerminalLine[] = [
    [
      {text: '$ ', cls: 'tPrompt'},
      {text: INSTALL_CMD, cls: 'tCmd'},
    ],
    [{text: "[orka'i]", cls: 'tDim'}, {text: ' detecting host ······ ubuntu 24.04'}],
    [{text: "[orka'i]", cls: 'tDim'}, {text: ' provisioning k3s control plane'}],
    [
      {text: "[orka'i]", cls: 'tDim'},
      {text: ' control plane ready in '},
      {text: '47s', cls: 'tAccent'},
    ],
    [{text: "[orka'i]", cls: 'tDim'}, {text: ' console ·· https://console.orkai.dev'}],
    [],
    [{text: t.hero.terminalReady, cls: 'tOk'}, {text: ` ▸ ${t.hero.terminalCreate}`}],
  ];

  return (
    <AnimatedTerminal
      label="install.sh"
      lines={lines}
      rightSlot={
        <CopyButton
          value={INSTALL_CMD}
          label={t.copy.copyCommand}
          copiedLabel={t.copy.copied}
          copyLabel={t.copy.copy}
        />
      }
    />
  );
}

function HeroSection({t}: {t: HomepageContent}) {
  return (
    <section className={styles.hero}>
      <div className={styles.heroGlow} aria-hidden="true" />
      <div className={styles.heroInner}>
        <div className={styles.heroCopy}>
          <FadeIn>
            <span className={styles.eyebrow}>{t.hero.eyebrow}</span>
          </FadeIn>
          <FadeIn delay={70}>
            <h1 className={styles.heroTitle}>
              {t.hero.titleLine1}
              <br />
              <span className={styles.heroTitleAccent}>{t.hero.titleLine2}</span>
            </h1>
          </FadeIn>
          <FadeIn delay={140}>
            <p className={styles.heroLead}>{t.hero.lead}</p>
          </FadeIn>
          <FadeIn delay={210}>
            <div className={styles.heroActions}>
              <Link to={INSTALL_URL} className={styles.buttonPrimary}>
                {t.hero.ctaPrimary}
              </Link>
              <a
                href={GITHUB_URL}
                className={styles.buttonSecondary}
                target="_blank"
                rel="noopener noreferrer">
                <MaterialIcon name="code" className={styles.buttonIcon} />
                {t.hero.ctaSecondary}
              </a>
            </div>
          </FadeIn>
          <FadeIn delay={280}>
            <ul className={styles.heroProof}>
              {t.hero.proof.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </FadeIn>
        </div>

        <FadeIn delay={160} className={styles.heroEvidence}>
          <InstallTerminal t={t} />
        </FadeIn>
      </div>
    </section>
  );
}

function ValueProps({t}: {t: HomepageContent}) {
  return (
    <section className={styles.valueProps}>
      <div className={styles.valuePropsInner}>
        {t.valueProps.map((prop, idx) => (
          <FadeIn key={prop.title} delay={idx * 80} className={styles.valueCell}>
            <MaterialIcon name={prop.icon} className={styles.valueIcon} />
            <h3 className={styles.valueTitle}>{prop.title}</h3>
            <p className={styles.valueBody}>{prop.body}</p>
          </FadeIn>
        ))}
      </div>
    </section>
  );
}

function WhySection({t}: {t: HomepageContent}) {
  return (
    <section className={styles.section}>
      <div className={styles.sectionInner}>
        <FadeIn>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionEyebrow}>{t.why.eyebrow}</span>
            <h2 className={styles.sectionTitle}>{t.why.title}</h2>
            <p className={styles.sectionLead}>{t.why.lead}</p>
          </div>
        </FadeIn>

        <div className={styles.contrastGrid}>
          {t.why.contrast.map((col, idx) => (
            <FadeIn key={col.heading} delay={idx * 100}>
              <div className={styles.contrastCol} data-tone={col.tone}>
                <div className={styles.contrastHead}>
                  <MaterialIcon
                    name={col.tone === 'us' ? 'check_circle' : 'lock'}
                    className={styles.contrastHeadIcon}
                  />
                  <span>{col.heading}</span>
                </div>
                <ul className={styles.contrastList}>
                  {col.points.map((p) => (
                    <li key={p}>{p}</li>
                  ))}
                </ul>
              </div>
            </FadeIn>
          ))}
        </div>
      </div>
    </section>
  );
}

const RESOURCES = [
  {key: 'deployment.apps/core-api', value: '3/3 ready', state: 'ready'},
  {key: 'service/core-api', value: 'ClusterIP'},
  {key: 'ingress/core-api', value: 'api.orkai.dev'},
  {key: 'cronjob/core-api-backup', value: '0 2 * * *'},
  {key: 'secret/core-api-env', value: 'Opaque · 6 keys'},
];

function ResourceMapSection({t}: {t: HomepageContent}) {
  return (
    <section className={`${styles.section} ${styles.sectionAlt}`}>
      <div className={styles.sectionInner}>
        <div className={styles.mapSplit}>
          <FadeIn className={styles.mapText}>
            <span className={styles.sectionEyebrow}>{t.resourceMap.eyebrow}</span>
            <h2 className={styles.sectionTitle}>{t.resourceMap.title}</h2>
            <p className={styles.sectionLead}>{t.resourceMap.lead}</p>
            <p className={styles.mapFootnote}>{t.resourceMap.footnote}</p>
          </FadeIn>

          <FadeIn delay={120} className={styles.mapTableWrap}>
            <div className={styles.specTable}>
              <div className={styles.specTableHead}>
                <span>{t.resourceMap.tableHeadResource}</span>
                <span>{t.resourceMap.tableHeadStatus}</span>
              </div>
              {RESOURCES.map((r) => (
                <div key={r.key} className={styles.specRow}>
                  <span className={styles.specKey}>{r.key}</span>
                  <span className={styles.specValue} data-state={r.state ?? ''}>
                    {r.state === 'ready' && <span className={styles.specDot} />}
                    {r.value}
                  </span>
                </div>
              ))}
            </div>
          </FadeIn>
        </div>
      </div>
    </section>
  );
}

function PagesSection({t}: {t: HomepageContent}) {
  const lines: TerminalLine[] = [
    [
      {text: '$ ', cls: 'tPrompt'},
      {text: "orka'i page deploy docs-site", cls: 'tCmd'},
    ],
    [{text: "[orka'i]", cls: 'tDim'}, {text: ' cloning main @ docs/'}],
    [
      {text: "[orka'i]", cls: 'tDim'},
      {text: ' syncing '},
      {text: '42 files', cls: 'tAccent'},
      {text: ' → s3://orkai-docs-a1b2c3'},
    ],
    [{text: "[orka'i]", cls: 'tDim'}, {text: ' invalidating CloudFront distribution'}],
    [
      {text: "[orka'i]", cls: 'tDim'},
      {text: ' live ▸ '},
      {text: 'https://d111111abcdef8.cloudfront.net', cls: 'tOk'},
    ],
  ];

  return (
    <section className={styles.section}>
      <div className={styles.sectionInner}>
        <FadeIn>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionEyebrow}>{t.pages.eyebrow}</span>
            <h2 className={styles.sectionTitle}>{t.pages.title}</h2>
            <p className={styles.sectionLead}>{t.pages.lead}</p>
          </div>
        </FadeIn>

        <FadeIn delay={80}>
          <ul className={styles.pagesList}>
            {t.pages.list.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </FadeIn>

        <FadeIn delay={120}>
          <a
            href={IMPL_PAGES_URL}
            className={styles.implLink}
            target="_blank"
            rel="noopener noreferrer">
            <span className={styles.implLinkLabel}>
              <MaterialIcon name="description" className={styles.buttonIcon} />
              {t.pages.implLabel}
            </span>
            <code className={styles.implPath}>{t.pages.implPath}</code>
          </a>
        </FadeIn>

        <FadeIn delay={160} className={styles.pagesTerminalWrap}>
          <AnimatedTerminal label="page-deploy" lines={lines} step={320} />
        </FadeIn>
      </div>
    </section>
  );
}

function ClusterSection({t}: {t: HomepageContent}) {
  const lines: TerminalLine[] = [
    [
      {text: '$ ', cls: 'tPrompt'},
      {text: 'kubectl get pods -n production', cls: 'tCmd'},
    ],
    [{text: 'core-api-7d9f8c', cls: 'tDim'}, {text: '      3/3   '}, {text: 'Running', cls: 'tOk'}],
    [{text: 'postgres-core-0', cls: 'tDim'}, {text: '      1/1   '}, {text: 'Running', cls: 'tOk'}],
    [],
    [
      {text: '$ ', cls: 'tPrompt'},
      {text: 'helm list -n production', cls: 'tCmd'},
    ],
    [{text: 'valkey-cache', cls: 'tDim'}, {text: '   deployed   valkey-8.0.1'}],
    [],
    [
      {text: '$ ', cls: 'tPrompt'},
      {text: "orka'i db create orders --engine postgres", cls: 'tCmd'},
    ],
    [{text: 'created', cls: 'tOk'}, {text: ' statefulset/orders + pvc (20Gi)'}],
    [{text: 'created', cls: 'tOk'}, {text: ' secret/orders-credentials'}],
  ];

  return (
    <section className={styles.section}>
      <div className={styles.sectionInner}>
        <div className={styles.clusterSplit}>
          <FadeIn className={styles.clusterTerminalWrap}>
            <AnimatedTerminal label="admin@k3s-01" lines={lines} step={300} />
          </FadeIn>

          <FadeIn delay={120} className={styles.clusterText}>
            <span className={styles.sectionEyebrow}>{t.cluster.eyebrow}</span>
            <h2 className={styles.sectionTitle}>{t.cluster.title}</h2>
            <p className={styles.sectionLead}>{t.cluster.lead}</p>
            <ul className={styles.clusterList}>
              {t.cluster.list.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </FadeIn>
        </div>
      </div>
    </section>
  );
}

function LedgerSection({t}: {t: HomepageContent}) {
  return (
    <section className={`${styles.section} ${styles.sectionAlt}`}>
      <div className={styles.sectionInner}>
        <FadeIn>
          <div className={styles.sectionHeader}>
            <span className={styles.sectionEyebrow}>{t.ledger.eyebrow}</span>
            <h2 className={styles.sectionTitle}>{t.ledger.title}</h2>
          </div>
        </FadeIn>

        <div className={styles.ledger}>
          {t.ledger.rows.map((row, idx) => (
            <FadeIn key={row.label} delay={idx * 70}>
              <div className={styles.ledgerRow}>
                <div className={styles.ledgerLabel}>
                  <span className={styles.ledgerLabelText}>[ {row.label} ]</span>
                  <h3 className={styles.ledgerTitle}>{row.title}</h3>
                </div>
                <ul className={styles.ledgerItems}>
                  {row.items.map((item) => (
                    <li key={item}>
                      <MaterialIcon name="chevron_right" className={styles.ledgerChevron} />
                      <span>{item}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </FadeIn>
          ))}
        </div>
      </div>
    </section>
  );
}

function CtaSection({t}: {t: HomepageContent}) {
  return (
    <section className={styles.cta}>
      <div className={styles.sectionInner}>
        <div className={styles.ctaInner}>
          <div className={styles.ctaCopy}>
            <span className={styles.sectionEyebrow}>{t.cta.eyebrow}</span>
            <h2 className={styles.ctaTitle}>{t.cta.title}</h2>
            <p className={styles.ctaLead}>{t.cta.lead}</p>
            <div className={styles.heroActions}>
              <Link to={INSTALL_URL} className={styles.buttonPrimary}>
                {t.cta.ctaPrimary}
              </Link>
              <Link to={DOCS_URL} className={styles.buttonSecondary}>
                {t.cta.ctaSecondary}
              </Link>
            </div>
          </div>

          <div className={styles.ctaInstall}>
            <div className={styles.ctaInstallHead}>
              <span>{t.cta.installLabel}</span>
              <CopyButton
                value={INSTALL_CMD}
                label={t.copy.copyCommand}
                copiedLabel={t.copy.copied}
                copyLabel={t.copy.copy}
              />
            </div>
            <code className={styles.ctaInstallCmd}>
              <span className={styles.tPrompt}>$</span> {INSTALL_CMD}
            </code>
            <a
              href={GITHUB_URL}
              className={styles.ctaGithub}
              target="_blank"
              rel="noopener noreferrer">
              <MaterialIcon name="star" className={styles.buttonIcon} />
              {t.cta.github}
            </a>
          </div>
        </div>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  const {siteConfig, i18n} = useDocusaurusContext();
  const t = getHomepageContent(i18n.currentLocale);
  const description =
    (siteConfig.customFields?.seoDescription as string | undefined) ?? t.meta.description;

  return (
    <Layout title={t.meta.title} description={description}>
      <main className={styles.landingPage}>
        <HeroSection t={t} />
        <ValueProps t={t} />
        <WhySection t={t} />
        <ResourceMapSection t={t} />
        <ClusterSection t={t} />
        <LedgerSection t={t} />
        <PagesSection t={t} />
        <CtaSection t={t} />
      </main>
    </Layout>
  );
}
