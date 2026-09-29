import { useCallback, useEffect, useMemo, useRef, type ReactNode } from 'react';

import { useMediaQuery, REDUCED_MOTION_QUERY } from '../../hooks/useMediaQuery';
import { useStore } from '../../lib/store';
import { fx } from '../../state/fx';
import { CrtContext, type Crt } from './crt';
import styles from './CrtScreen.module.css';

/** Ambient glitch intensity, 0 to 1. */
const INTENSITY = 0.7;

/**
 * The CRT monitor every screen is shown on. Ambient motion (flicker, glitch
 * bursts) runs on the Web Animations API; everything stops in calm mode and
 * for people who prefer reduced motion.
 *
 * Content inside is transformed briefly during glitches, so anything that is
 * position: fixed must be rendered in a portal, not inside the content.
 */
export function CrtScreen({ children }: { children: ReactNode }) {
  const level = useStore(fx);
  const reduced = useMediaQuery(REDUCED_MOTION_QUERY);
  const calm = level === 'calm' || reduced;
  const content = useRef<HTMLDivElement>(null);
  const chroma = useRef<HTMLDivElement>(null);
  const tear = useRef<HTMLDivElement>(null);

  useEffect(() => {
    document.documentElement.dataset.fx = calm ? 'calm' : 'full';
  }, [calm]);

  const burst = useCallback(
    (strength: number) => {
      const c = content.current;
      const ch = chroma.current;
      const t = tear.current;
      if (calm || !c || !ch || !t || typeof c.animate !== 'function') return;
      t.style.top = `${String(Math.round(8 + Math.random() * 80))}vh`;
      c.animate(
        [
          { transform: 'none' },
          {
            transform: `translateX(${String(5 * strength)}px) skewX(${String(-0.9 * strength)}deg)`,
            offset: 0.25,
          },
          {
            transform: `translateX(${String(-4 * strength)}px) skewX(${String(0.7 * strength)}deg)`,
            offset: 0.5,
          },
          { transform: 'none' },
        ],
        { duration: 200, easing: 'steps(4, end)' },
      );
      ch.animate([{ opacity: 0 }, { opacity: 0.6 * strength, offset: 0.25 }, { opacity: 0 }], {
        duration: 220,
      });
      t.animate([{ opacity: 0.9 }, { opacity: 0.9, offset: 0.9 }, { opacity: 0 }], { duration: 200 });
    },
    [calm],
  );

  useEffect(() => {
    if (calm) return;
    let flickerTimer: ReturnType<typeof setTimeout>;
    let burstTimer: ReturnType<typeof setTimeout>;
    const flicker = () => {
      const c = content.current;
      if (c && typeof c.animate === 'function' && document.visibilityState === 'visible') {
        c.animate([{ opacity: 1 }, { opacity: 0.88 + Math.random() * 0.1 }, { opacity: 1 }], {
          duration: 100,
        });
      }
      flickerTimer = setTimeout(flicker, 500 + Math.random() * 2500);
    };
    const loop = () => {
      if (document.visibilityState === 'visible') burst(INTENSITY);
      burstTimer = setTimeout(loop, 2200 + Math.random() * 5000);
    };
    flickerTimer = setTimeout(flicker, 800);
    burstTimer = setTimeout(loop, 1600);
    return () => {
      clearTimeout(flickerTimer);
      clearTimeout(burstTimer);
    };
  }, [calm, burst]);

  const api = useMemo<Crt>(() => ({ glitch: (s = 1.4) => burst(s) }), [burst]);

  return (
    <CrtContext.Provider value={api}>
      <div className={calm ? styles.calm : undefined}>
        <div className={styles.bloom} aria-hidden="true" />
        <div ref={content} className={styles.content}>
          {children}
        </div>
        <div className={styles.band} aria-hidden="true" />
        <div ref={chroma} className={styles.chroma} aria-hidden="true" />
        <div ref={tear} className={styles.tear} aria-hidden="true" />
        <div className={styles.scan} aria-hidden="true" />
        <div className={styles.vignette} aria-hidden="true" />
      </div>
    </CrtContext.Provider>
  );
}
