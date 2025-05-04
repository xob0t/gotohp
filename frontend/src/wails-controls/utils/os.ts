// src/components/wails-controls/utils/os.ts
import { System } from "@wailsio/runtime"

let osType: string | undefined = undefined

export function getOsType(): Promise<string> {
  if (osType) {
    return Promise.resolve(osType)
  }
  
  return new Promise((resolve) => {
    if (System.IsMac()) {
      osType = "darwin"
    } else if (System.IsLinux()) {
      osType = "linux"
    } else if (System.IsWindows()) {
      osType = "windows"
    } else {
      osType = "unknown"
    }
    resolve(osType)
  })
}