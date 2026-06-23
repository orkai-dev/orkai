import type {ReactNode} from 'react';
import Link from '@docusaurus/Link';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import useBaseUrl from '@docusaurus/useBaseUrl';

import {getFooterStrings} from '../../i18n/footer';
import styles from './styles.module.css';

type FooterItem = {
  label: string;
  to?: string;
  href?: string;
};

type FooterColumn = {
  title?: string;
  items: FooterItem[];
};

function FooterLink({item}: {item: FooterItem}): ReactNode {
  const external = Boolean(item.href);
  return (
    <Link
      className={styles.link}
      {...(item.href ? {href: item.href} : {to: item.to})}
      {...(external ? {target: '_blank', rel: 'noopener noreferrer'} : {})}>
      {item.label}
    </Link>
  );
}

type FooterConfig = {
  links?: FooterColumn[];
  copyright?: string;
};

export default function Footer(): ReactNode {
  const {siteConfig, i18n} = useDocusaurusContext();
  const footerStrings = getFooterStrings(i18n.currentLocale);
  const logoUrl = useBaseUrl('/img/logo.svg');
  const footer = siteConfig.themeConfig?.footer as FooterConfig | undefined;
  if (!footer) {
    return null;
  }

  const columns = footer.links ?? [];
  const copyright = footer.copyright;

  return (
    <footer className={styles.footer}>
      <div className={styles.inner}>
        <div className={styles.brand}>
          <Link to="/" className={styles.brandTop}>
            <img src={logoUrl} alt="orka'i" className={styles.brandLogo} />
            <span className={styles.brandName}>orka&apos;i</span>
          </Link>
          <p className={styles.brandTagline}>{footerStrings.brandTagline}</p>
          <span className={styles.brandMeta}>{footerStrings.brandMeta}</span>
        </div>

        <nav className={styles.columns} aria-label="Footer">
          {columns.map((col, i) => (
            <div key={col.title ?? i} className={styles.column}>
              {col.title ? <span className={styles.columnTitle}>{col.title}</span> : null}
              <ul className={styles.columnList}>
                {col.items.map((item) => (
                  <li key={item.label}>
                    <FooterLink item={item} />
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </nav>
      </div>

      <div className={styles.bar}>
        <div className={styles.barInner}>
          {copyright ? <span className={styles.copyright}>{copyright}</span> : null}
          <span className={styles.barMeta}>
            <span className={styles.statusDot} aria-hidden="true" />
            {footerStrings.builtOn}
          </span>
        </div>
      </div>
    </footer>
  );
}
