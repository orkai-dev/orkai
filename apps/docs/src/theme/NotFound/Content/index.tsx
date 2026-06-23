import type {ReactNode} from 'react';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Link from '@docusaurus/Link';

import {getNotFoundContent} from '../../../i18n/notFound';
import styles from './styles.module.css';

export default function NotFoundContent(): ReactNode {
  const {i18n} = useDocusaurusContext();
  const t = getNotFoundContent(i18n.currentLocale);

  return (
    <main className={styles.page}>
      <div className={styles.inner}>
        <span className={styles.eyebrow}>{t.eyebrow}</span>
        <h1 className={styles.title}>{t.title}</h1>
        <p className={styles.lead}>{t.lead}</p>

        <div className={styles.terminal}>
          <div className={styles.terminalHeader}>
            <span className={styles.terminalLabel}>{t.terminalLabel}</span>
          </div>
          <pre className={styles.terminalBody}>
            <code>
              <span className={styles.tPrompt}>$</span>{' '}
              <span className={styles.tCmd}>kubectl get page unknown</span>
              {'\n'}
              <span className={styles.tError}>{t.terminalOutput}</span>
            </code>
          </pre>
        </div>

        <div className={styles.actions}>
          <Link to="/" className={styles.buttonPrimary}>
            {t.ctaHome}
          </Link>
          <Link to="/docs/intro" className={styles.buttonSecondary}>
            {t.ctaDocs}
          </Link>
        </div>
      </div>
    </main>
  );
}
