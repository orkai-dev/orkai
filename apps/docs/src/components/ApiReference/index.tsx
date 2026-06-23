import styles from './styles.module.css';

export default function ApiReference() {
  return (
    <iframe
      className={styles.frame}
      title="Orkai API Reference"
      src="/redoc.html"
      loading="lazy"
    />
  );
}
