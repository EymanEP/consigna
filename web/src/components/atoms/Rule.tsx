import styles from './Rule.module.css';

/** A hairline that fills the remaining width of a flex row. */
export function Rule() {
  return <span className={styles.rule} aria-hidden="true" />;
}
