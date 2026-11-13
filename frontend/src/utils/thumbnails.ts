import { useMediaQuery } from '@vueuse/core'
import { computed, reactive, ref, watch } from 'vue'
import { debugLog } from './debug'

export type MediaKind = 'image' | 'video'

const STORAGE_KEY = 'gotohp.thumbnails'
/** Thumbnails are drawn only when the window is at least this wide. */
const WIDE_QUERY = '(min-width: 640px)'
/** Longest edge of a stored thumbnail, in pixels. Shown at up to 96 CSS px. */
const THUMB_SIZE = 192
/** Originals above this size are not decoded; the row keeps its icon. */
const MAX_IMAGE_BYTES = 80 * 1024 * 1024
const VIDEO_TIMEOUT_MS = 8000
/** Finished thumbnails kept in memory (a few KB each). */
const MAX_CACHED = 400

const IMAGE_EXTENSIONS = new Set(['jpg', 'jpeg', 'png', 'gif', 'webp', 'avif', 'bmp'])
const VIDEO_EXTENSIONS = new Set(['mp4', 'm4v', 'mov', 'webm'])

function readStoredFlag(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) !== '0'
  } catch {
    return true
  }
}

/** User preference, on by default. */
export const thumbnailsEnabled = ref(readStoredFlag())

watch(thumbnailsEnabled, (enabled) => {
  try {
    localStorage.setItem(STORAGE_KEY, enabled ? '1' : '0')
  } catch {
    // Without storage the choice lasts for this session.
  }
})

const isWideWindow = useMediaQuery(WIDE_QUERY)

/** True when thumbnails should be generated and shown right now. */
export const thumbnailsActive = computed(() => thumbnailsEnabled.value && isWideWindow.value)

/** The kind of preview a path can have, judged by its extension. */
export function mediaKind(path: string): MediaKind | null {
  const extension = path.slice(path.lastIndexOf('.') + 1).toLowerCase()
  if (IMAGE_EXTENSIONS.has(extension)) return 'image'
  if (VIDEO_EXTENSIONS.has(extension)) return 'video'
  return null
}

// path -> data URL of the small thumbnail, or null when none could be made
const cache = reactive(new Map<string, string | null>())
const queued = new Set<string>()
const queue: string[] = []
let working = false

export const thumbnailStats = reactive({ generated: 0, failed: 0 })

/** The cached thumbnail for path, if one has been generated. */
export function cachedThumbnail(path: string): string | null {
  return cache.get(path) ?? null
}

/**
 * Asks for a thumbnail of path. Work is done one file at a time so previews never
 * compete with the upload for more than a single decode.
 */
export function requestThumbnail(path: string) {
  if (!path || cache.has(path) || queued.has(path) || !mediaKind(path)) return
  queued.add(path)
  queue.push(path)
  void drain()
}

async function drain() {
  if (working) return
  working = true
  try {
    while (queue.length > 0) {
      // Newest first: the files on screen are the ones requested last.
      const path = queue.pop() as string
      queued.delete(path)
      if (!thumbnailsActive.value) continue

      let thumbnail: string | null = null
      try {
        thumbnail = await generate(path)
        thumbnailStats.generated += 1
      } catch (error) {
        thumbnailStats.failed += 1
        debugLog('info', 'frontend', 'thumbnail unavailable', {
          path,
          reason: error instanceof Error ? error.message : String(error),
        })
      }
      store(path, thumbnail)
    }
  } finally {
    working = false
  }
}

function store(path: string, thumbnail: string | null) {
  cache.set(path, thumbnail)
  if (cache.size > MAX_CACHED) {
    // Maps iterate in insertion order, so this drops the oldest entry.
    const oldest = cache.keys().next().value
    if (oldest !== undefined) cache.delete(oldest)
  }
}

function previewUrl(path: string): string {
  return `/__preview?path=${encodeURIComponent(path)}`
}

function generate(path: string): Promise<string> {
  return mediaKind(path) === 'video' ? generateFromVideo(path) : generateFromImage(path)
}

function fitWithin(width: number, height: number): [number, number] {
  const scale = Math.min(1, THUMB_SIZE / Math.max(width, height))
  return [Math.max(1, Math.round(width * scale)), Math.max(1, Math.round(height * scale))]
}

function drawToDataUrl(source: CanvasImageSource, width: number, height: number): string {
  const [targetWidth, targetHeight] = fitWithin(width, height)
  const canvas = document.createElement('canvas')
  canvas.width = targetWidth
  canvas.height = targetHeight
  const context = canvas.getContext('2d')
  if (!context) throw new Error('canvas unavailable')
  context.drawImage(source, 0, 0, targetWidth, targetHeight)
  return canvas.toDataURL('image/jpeg', 0.72)
}

async function generateFromImage(path: string): Promise<string> {
  const response = await fetch(previewUrl(path))
  if (!response.ok) throw new Error(`preview request failed (${response.status})`)
  const blob = await response.blob()
  if (blob.size > MAX_IMAGE_BYTES) throw new Error('file too large to preview')

  // The decoded original is released as soon as the small copy is drawn.
  const bitmap = await createImageBitmap(blob)
  try {
    return drawToDataUrl(bitmap, bitmap.width, bitmap.height)
  } finally {
    bitmap.close()
  }
}

function generateFromVideo(path: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const video = document.createElement('video')
    let settled = false

    const finish = (result: string | Error) => {
      if (settled) return
      settled = true
      clearTimeout(timer)
      video.removeAttribute('src')
      video.load() // stops any further download of the file
      if (result instanceof Error) reject(result)
      else resolve(result)
    }

    const timer = setTimeout(() => finish(new Error('video preview timed out')), VIDEO_TIMEOUT_MS)

    video.muted = true
    video.playsInline = true
    video.preload = 'metadata'
    video.addEventListener('loadedmetadata', () => {
      // Skip a possible black first frame without reading far into the file.
      video.currentTime = Math.min(0.5, (video.duration || 1) / 2)
    })
    video.addEventListener('seeked', () => {
      try {
        if (!video.videoWidth || !video.videoHeight) throw new Error('video has no picture')
        finish(drawToDataUrl(video, video.videoWidth, video.videoHeight))
      } catch (error) {
        finish(error instanceof Error ? error : new Error(String(error)))
      }
    })
    video.addEventListener('error', () => finish(new Error('video format not supported by the webview')))
    video.src = previewUrl(path)
  })
}
