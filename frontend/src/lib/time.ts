const timePattern = /^(\d+(\.\d+)?|(\d+:)?[0-5]?\d:[0-5]\d(\.\d+)?)$/

export function isValidTime(value: string): boolean {
  return value.trim() === '' || timePattern.test(value.trim())
}
