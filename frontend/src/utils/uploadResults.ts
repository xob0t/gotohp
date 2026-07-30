export interface UploadSuccess {
  path: string
  mediaKey: string
}

export interface SkippedUploadResult {
  paths: string[]
  code: string
  reason: string
}

export interface UploadResults {
  success: UploadSuccess[]
  fail: string[]
  skipped: SkippedUploadResult[]
  warnings: SkippedUploadResult[]
}

export interface UploadResultEvent {
  MediaKey: string
  IsError: boolean
  Skipped: boolean
  ErrorMessage: string
  SkipCode: string
  SkipReason: string
  Path: string
  Paths: string[]
}

export function recordUploadResult(results: UploadResults, event: UploadResultEvent): number {
  if (event.IsError) {
    results.fail.push(event.ErrorMessage ? `${event.Path}: ${event.ErrorMessage}` : event.Path)
    return 1
  }

  if (event.Skipped) {
    const paths = event.Paths?.length ? event.Paths : [event.Path]
    results.skipped.push({
      paths: paths.filter(Boolean),
      code: event.SkipCode || 'skipped',
      reason: event.SkipReason || 'Skipped by upload policy',
    })
    return 1
  }

  results.success.push({ path: event.Path, mediaKey: event.MediaKey })
  return 1
}
