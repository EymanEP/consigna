import type { SVGProps } from 'react';

// Stroke icons drawn on a 24px grid, square caps to match the CRT style.
const paths = {
  download: 'M12 4v11 M7 11l5 5 5-5 M5 20h14',
  drop: 'M12 3v11 M7.5 9.5 12 14l4.5-4.5 M3.5 15.5v4h17v-4',
  add: 'M12 4v10 M8 10l4 4 4-4 M4 16v4h16v-4',
  up: 'M12 20V5 M6 11l6-6 6 6',
  pencil: 'M4 20h4L20 8l-4-4L4 16v4z',
  close: 'M6 6l12 12 M18 6 6 18',
  check: 'M5 12.5l4.5 4.5L19 7',
  copy: 'M9 9h11v11H9z M5 15H4V4h11v1',
  trash: 'M4 7h16 M9 7V4h6v3 M6 7l1 13h10l1-13',
  monitor: 'M3 4h18v13H3z M8 21h8',
  phone: 'M7 3h10v18H7z M11 18h2',
  tablet: 'M5 3h14v18H5z M11 18h2',
  laptop: 'M5 5h14v10H5z M2 19h20',
  wifi: 'M2.5 9a14 14 0 0 1 19 0 M5.5 12.5a9.5 9.5 0 0 1 13 0 M8.5 16a5 5 0 0 1 7 0 M12 19.5h.01',
  ethernet: 'M4 5h16v11H4z M8 16v3 M12 16v3 M16 16v3 M8 9v3 M12 9v3 M16 9v3',
  vpn: 'M12 3l7 3v5c0 5-3 8-7 10-4-2-7-5-7-10V6z',
  warning: 'M12 3 2 20h20z M12 10v4 M12 17h.01',
  refresh: 'M20 11a8 8 0 1 0-2.3 5.7 M20 5v6h-6',
  settings: 'M4 7h10 M18 7h2 M4 17h2 M10 17h10 M14 5v4 M6 15v4',
  users:
    'M9 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7z M2.5 20c.8-3.5 3.3-5.5 6.5-5.5s5.7 2 6.5 5.5 M16 4.5a3.5 3.5 0 0 1 0 6.5 M18.5 14.5c1.6.9 2.6 2.8 3 5.5',
  power: 'M12 3v8 M6.3 6.8a8 8 0 1 0 11.4 0',
  arrowRight: 'M4 12h15 M13 6l6 6-6 6',
  chevronDown: 'M6 9l6 6 6-6',
  qr: 'M4 4h6v6H4z M14 4h6v6h-6z M4 14h6v6H4z M14 14h2v2h-2z M18 18h2v2h-2z M14 18h2 M18 14h2',
  spark: 'M12 3v4 M12 17v4 M3 12h4 M17 12h4',
  archive: 'M3 4h18v4H3z M5 8v12h14V8 M10 12h4',
} as const;

export type IconName = keyof typeof paths;

interface IconProps extends Omit<SVGProps<SVGSVGElement>, 'name'> {
  name: IconName;
  size?: number;
  strokeWidth?: number;
}

/** A decorative icon. Give its button an aria-label for meaning. */
export function Icon({ name, size = 18, strokeWidth = 1.8, ...rest }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={strokeWidth}
      strokeLinecap="square"
      aria-hidden="true"
      focusable="false"
      {...rest}
    >
      {paths[name].split(' M').map((d, i) => (
        <path key={i} d={i === 0 ? d : `M${d}`} />
      ))}
    </svg>
  );
}
