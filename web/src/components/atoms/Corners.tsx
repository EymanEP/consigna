import styles from './Corners.module.css';

/** The four accent brackets framing a target. Place inside a positioned box. */
export function Corners() {
  return (
    <>
      <span className={`${styles.corner} ${styles.tl}`} aria-hidden="true" />
      <span className={`${styles.corner} ${styles.tr}`} aria-hidden="true" />
      <span className={`${styles.corner} ${styles.bl}`} aria-hidden="true" />
      <span className={`${styles.corner} ${styles.br}`} aria-hidden="true" />
    </>
  );
}
