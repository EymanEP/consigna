import { useRef, type KeyboardEvent } from 'react';

import styles from './Tabs.module.css';

export interface TabItem<K extends string> {
  key: K;
  label: string;
}

interface TabsProps<K extends string> {
  items: readonly TabItem<K>[];
  active: K;
  onChange: (key: K) => void;
  label: string;
  /** Prefix for tab/panel ids: tabs are `${idPrefix}-tab-${key}`. */
  idPrefix: string;
}

/** Accessible tabs (arrow keys move between them). Render panels with tabPanelProps. */
export function Tabs<K extends string>({ items, active, onChange, label, idPrefix }: TabsProps<K>) {
  const refs = useRef(new Map<K, HTMLButtonElement>());

  const onKey = (e: KeyboardEvent, index: number) => {
    let next = -1;
    if (e.key === 'ArrowRight') next = (index + 1) % items.length;
    if (e.key === 'ArrowLeft') next = (index - 1 + items.length) % items.length;
    if (e.key === 'Home') next = 0;
    if (e.key === 'End') next = items.length - 1;
    const item = items[next];
    if (!item) return;
    e.preventDefault();
    onChange(item.key);
    refs.current.get(item.key)?.focus();
  };

  return (
    <div role="tablist" aria-label={label} className={styles.list}>
      {items.map((item, i) => (
        <button
          key={item.key}
          ref={(el) => {
            if (el) refs.current.set(item.key, el);
            else refs.current.delete(item.key);
          }}
          type="button"
          role="tab"
          id={`${idPrefix}-tab-${item.key}`}
          aria-selected={item.key === active}
          aria-controls={`${idPrefix}-panel-${item.key}`}
          tabIndex={item.key === active ? 0 : -1}
          className={styles.tab}
          onClick={() => onChange(item.key)}
          onKeyDown={(e) => onKey(e, i)}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}
