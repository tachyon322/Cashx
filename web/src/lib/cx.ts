/** Склейка условных Tailwind-классов (аналог clsx/twMerge без зависимостей). */
export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(' ')
}