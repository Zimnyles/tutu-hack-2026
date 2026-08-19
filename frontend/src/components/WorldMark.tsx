export function WorldMark({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="22 22 84 84" role="img" aria-label="Планета Открывай">
      <defs>
        <clipPath id="world-mark-globe">
          <circle cx="64" cy="64" r="42" />
        </clipPath>
        <radialGradient id="world-mark-sphere" cx="34%" cy="28%" r="82%">
          <stop offset="0" stopColor="#ffffff" stopOpacity=".34" />
          <stop offset=".5" stopColor="#ffffff" stopOpacity="0" />
          <stop offset="1" stopColor="#0d0b68" stopOpacity=".45" />
        </radialGradient>
      </defs>
      <circle cx="64" cy="64" r="42" fill="#7d71ff" />
      <g clipPath="url(#world-mark-globe)">
        <g fill="#00c95e">
          <path d="M26 40c8-7 19-9 28-4 6 3 8 10 15 11 8 1 15 6 13 13-2 6-10 8-17 6-9-2-15 3-24 2-10-1-19-6-21-15-1-6 1-10 6-13Z" />
          <path d="M52 75c9-4 18 0 20 8 2 7-2 13 1 20 3 8-3 14-10 14-9 0-14-8-15-17-1-9-3-19 4-25Z" />
          <path d="M84 56c7-4 16-2 19 5 3 6-1 13-8 15-7 2-14-2-15-9-1-5 1-9 4-11Z" />
        </g>
        <g fill="none" stroke="#edefff" strokeOpacity=".4" strokeWidth="2.5">
          <path d="M22 64h84" />
          <ellipse cx="64" cy="64" rx="20" ry="42" />
        </g>
        <circle cx="64" cy="64" r="42" fill="url(#world-mark-sphere)" />
      </g>
      <circle cx="84" cy="40" r="8" fill="#ff872e" stroke="#0d0b68" strokeWidth="4" />
    </svg>
  )
}
